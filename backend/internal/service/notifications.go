package service

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/Jihed-khelifi/staryield/backend/internal/apperror"
	"github.com/Jihed-khelifi/staryield/backend/internal/clock"
	"github.com/Jihed-khelifi/staryield/backend/internal/database"
	"github.com/Jihed-khelifi/staryield/backend/internal/domain"
	"github.com/Jihed-khelifi/staryield/backend/internal/realtime"
	"github.com/Jihed-khelifi/staryield/backend/internal/repository"
	"github.com/Jihed-khelifi/staryield/backend/internal/reqctx"
)

// EventNotificationCreated is pushed to a user when a notification arrives.
const EventNotificationCreated = "notification.created"

// Notification kinds (rendered by clients from Kind + Data).
const (
	NotifyChatRequested   = "chat.requested"
	NotifyChatMissed      = "chat.missed"
	NotifyReviewPrompt    = "chat.review_prompt"
	NotifyReviewReceived  = "review.received"
	NotifyRequestCreated  = "request.created"
	NotifyRequestUpdated  = "request.updated"
	NotifyPurchase        = "purchase.succeeded"
	NotifyCreditsAdjusted = "credits.adjusted"
	NotifyWithdrawal      = "withdrawal.updated"
	NotifyBroadcast       = "broadcast"
)

// NotificationSpec describes a notification to deliver.
type NotificationSpec struct {
	Kind        string
	Data        map[string]any
	Link        string
	BroadcastID *uuid.UUID
}

// Notifier delivers notifications asynchronously. Implementations must
// never block the caller's request path.
type Notifier interface {
	Notify(ctx context.Context, spec NotificationSpec, userIDs ...uuid.UUID)
}

// NoopNotifier discards notifications.
type NoopNotifier struct{}

// Notify implements Notifier.
func (NoopNotifier) Notify(context.Context, NotificationSpec, ...uuid.UUID) {}

// NotificationView is a notification as returned to its owner.
type NotificationView struct {
	ID        uuid.UUID      `json:"id"`
	Kind      string         `json:"kind"`
	Data      map[string]any `json:"data"`
	Link      string         `json:"link"`
	ReadAt    *time.Time     `json:"read_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func notificationView(n *domain.Notification) NotificationView {
	data := map[string]any(n.Data)
	if data == nil {
		data = map[string]any{}
	}
	return NotificationView{ID: n.ID, Kind: n.Kind, Data: data, Link: n.Link, ReadAt: n.ReadAt, CreatedAt: n.CreatedAt}
}

// DispatcherConfig sizes the async notification pipeline.
type DispatcherConfig struct {
	BufferSize    int
	BatchSize     int
	FlushInterval time.Duration
}

// Dispatcher persists and pushes notifications in the background: Notify
// only enqueues into a bounded channel (dropping, with a log, when full), so
// endpoints that trigger notifications are never slowed or failed by them.
type Dispatcher struct {
	repo    repository.NotificationRepository
	events  EventPublisher
	log     *zap.Logger
	clock   clock.Clock
	cfg     DispatcherConfig
	ch      chan domain.Notification
	stop    chan struct{}
	once    sync.Once
	wg      sync.WaitGroup
	dropped atomic.Int64
	written atomic.Int64
}

// NewDispatcher returns a dispatcher; call Start to begin delivery.
func NewDispatcher(repo repository.NotificationRepository, events EventPublisher, log *zap.Logger, clk clock.Clock, cfg DispatcherConfig) *Dispatcher {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 1000
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 250 * time.Millisecond
	}
	return &Dispatcher{repo: repo, events: events, log: log, clock: clk, cfg: cfg,
		ch: make(chan domain.Notification, cfg.BufferSize), stop: make(chan struct{})}
}

// Notify implements Notifier.
func (d *Dispatcher) Notify(ctx context.Context, spec NotificationSpec, userIDs ...uuid.UUID) {
	if len(userIDs) == 0 || spec.Kind == "" {
		return
	}
	now := d.clock.Now()
	seen := make(map[uuid.UUID]bool, len(userIDs))
	for _, uid := range userIDs {
		if uid == uuid.Nil || seen[uid] {
			continue
		}
		seen[uid] = true
		n := domain.Notification{ID: domain.NewID(), UserID: uid, Kind: spec.Kind, Data: domain.JSONMap(copyMap(spec.Data)),
			Link: truncateString(spec.Link, 300), BroadcastID: spec.BroadcastID, CreatedAt: now}
		select {
		case d.ch <- n:
		default:
			if c := d.dropped.Add(1); c == 1 || c%1000 == 0 {
				d.log.Error("notifications: buffer full, dropping", zap.Int64("dropped_total", c), zap.String("kind", spec.Kind))
			}
		}
	}
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// Stats returns written and dropped counters.
func (d *Dispatcher) Stats() (written, dropped int64) { return d.written.Load(), d.dropped.Load() }

// Start launches the background writer.
func (d *Dispatcher) Start() {
	d.wg.Add(1)
	go d.run()
}

// Stop drains the queue and waits for the writer (bounded by ctx).
func (d *Dispatcher) Stop(ctx context.Context) {
	d.once.Do(func() { close(d.stop) })
	done := make(chan struct{})
	go func() { d.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-ctx.Done():
		d.log.Warn("notifications: shutdown before drain completed", zap.Int("buffered", len(d.ch)))
	}
}

func (d *Dispatcher) run() {
	defer d.wg.Done()
	ticker := time.NewTicker(d.cfg.FlushInterval)
	defer ticker.Stop()
	batch := make([]domain.Notification, 0, d.cfg.BatchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		d.deliver(batch)
		batch = batch[:0]
	}
	for {
		select {
		case n := <-d.ch:
			batch = append(batch, n)
			if len(batch) >= d.cfg.BatchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-d.stop:
			for {
				select {
				case n := <-d.ch:
					batch = append(batch, n)
					if len(batch) >= d.cfg.BatchSize {
						flush()
					}
					continue
				default:
				}
				break
			}
			flush()
			return
		}
	}
}

// deliver persists a batch, then pushes each notification to its owner.
func (d *Dispatcher) deliver(batch []domain.Notification) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rows := append([]domain.Notification(nil), batch...)
	if err := d.repo.InsertBatch(ctx, rows); err != nil {
		d.log.Error("notifications: insert failed", zap.Error(err), zap.Int("count", len(rows)))
		d.dropped.Add(int64(len(rows)))
		return
	}
	d.written.Add(int64(len(rows)))
	for i := range rows {
		v := notificationView(&rows[i])
		if err := d.events.Publish(ctx, EventNotificationCreated, map[string]any{"notification": v}, rows[i].UserID); err != nil {
			d.log.Warn("notifications: publish failed", zap.Error(err))
		}
	}
}

// NotificationService serves a user's notifications and admin broadcasts.
type NotificationService struct {
	repo       repository.NotificationRepository
	growth     repository.GrowthRepository
	presence   realtime.Presence
	events     EventPublisher
	audit      repository.AuditRepository
	clock      clock.Clock
	ipSecret   string
	log        *zap.Logger
	wg         sync.WaitGroup
	maxTargets int
}

// NewNotificationService wires the notification service.
func NewNotificationService(repo repository.NotificationRepository, growth repository.GrowthRepository, presence realtime.Presence,
	events EventPublisher, audit repository.AuditRepository, clk clock.Clock, ipSecret string, log *zap.Logger) *NotificationService {
	return &NotificationService{repo: repo, growth: growth, presence: presence, events: events, audit: audit, clock: clk,
		ipSecret: ipSecret, log: log, maxTargets: 1000}
}

// List returns the caller's notifications, newest first.
func (s *NotificationService) List(ctx context.Context, userID uuid.UUID, cursor string, limit int) (Page[NotificationView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[NotificationView]{}, err
	}
	rows, err := s.repo.List(ctx, userID, c, limit)
	if err != nil {
		return Page[NotificationView]{}, apperror.Internal(err)
	}
	page := Page[NotificationView]{Items: make([]NotificationView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, notificationView(&rows[i]))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// UnreadCount returns the number of unread notifications.
func (s *NotificationService) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	n, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return n, nil
}

// MarkRead marks some (ids) or all notifications as read and returns the
// remaining unread count.
func (s *NotificationService) MarkRead(ctx context.Context, userID uuid.UUID, ids []uuid.UUID, all bool) (int64, error) {
	if !all && len(ids) == 0 {
		return 0, apperror.Validation(map[string]string{"ids": "Choose notifications to mark as read."})
	}
	if len(ids) > 100 {
		return 0, apperror.Validation(map[string]string{"ids": "Mark at most 100 notifications at once."})
	}
	now := s.clock.Now()
	var err error
	if all {
		err = s.repo.MarkAllRead(ctx, userID, now)
	} else {
		err = s.repo.MarkRead(ctx, userID, ids, now)
	}
	if err != nil {
		return 0, apperror.Internal(err)
	}
	return s.UnreadCount(ctx, userID)
}

// BroadcastInput is an admin message to users.
type BroadcastInput struct {
	Audience   string            `json:"audience"`
	OnlineOnly bool              `json:"online_only"`
	UserIDs    []string          `json:"user_ids"`
	Emails     []string          `json:"emails"`
	CampaignID *uuid.UUID        `json:"campaign_id"`
	Title      map[string]string `json:"title"`
	Body       map[string]string `json:"body"`
	Link       string            `json:"link"`
}

// BroadcastView is a broadcast as seen by administrators.
type BroadcastView struct {
	ID         uuid.UUID         `json:"id"`
	Audience   string            `json:"audience"`
	OnlineOnly bool              `json:"online_only"`
	UserIDs    []string          `json:"user_ids,omitempty"`
	CampaignID *uuid.UUID        `json:"campaign_id,omitempty"`
	Title      map[string]string `json:"title"`
	Body       map[string]string `json:"body"`
	Link       string            `json:"link"`
	Status     string            `json:"status"`
	Recipients int               `json:"recipients"`
	CreatedBy  *uuid.UUID        `json:"created_by,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	SentAt     *time.Time        `json:"sent_at,omitempty"`
}

func localizedMap(m domain.JSONMap) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

func broadcastView(b *domain.Broadcast) BroadcastView {
	return BroadcastView{ID: b.ID, Audience: b.Audience, OnlineOnly: b.OnlineOnly, UserIDs: []string(b.TargetIDs), CampaignID: b.CampaignID,
		Title: localizedMap(b.Title), Body: localizedMap(b.Body), Link: b.Link, Status: b.Status, Recipients: b.Recipients,
		CreatedBy: b.CreatedBy, CreatedAt: b.CreatedAt, SentAt: b.SentAt}
}

// appPath validates a locale-less in-app link ("/chatroom?reader=x").
var appPath = regexp.MustCompile(`^/[A-Za-z0-9\-._~/?&=%#+]*$`)

// ValidAppLink reports whether link is a safe relative app path.
func ValidAppLink(link string) bool {
	return link == "" || (len(link) <= 300 && appPath.MatchString(link) && !strings.HasPrefix(link, "//") && !strings.Contains(link, "\\"))
}

func cleanLocalized(in map[string]string, field string, max int, fields map[string]string) domain.JSONMap {
	out := domain.JSONMap{}
	for _, loc := range []string{"en", "fr", "es"} {
		v := CleanText(in[loc])
		if utf8.RuneCountInString(v) > max {
			fields[field+"."+loc] = "This text is too long."
			continue
		}
		if v != "" {
			out[loc] = v
		}
	}
	for k := range in {
		if k != "en" && k != "fr" && k != "es" {
			fields[field+"."+k] = "Unsupported language."
		}
	}
	if out["en"] == nil {
		fields[field+".en"] = "The English version is required."
	}
	return out
}

// Broadcast stores a broadcast and delivers it in the background.
func (s *NotificationService) Broadcast(ctx context.Context, adminID uuid.UUID, in BroadcastInput) (*BroadcastView, error) {
	fields := map[string]string{}
	switch in.Audience {
	case "all", "clients", "psychics", "users":
	default:
		fields["audience"] = "Choose all, clients, psychics or users."
	}
	title := cleanLocalized(in.Title, "title", 120, fields)
	body := cleanLocalized(in.Body, "body", 1000, fields)
	link := strings.TrimSpace(in.Link)
	if !ValidAppLink(link) {
		fields["link"] = "Use an app path that starts with /."
	}
	var targets []uuid.UUID
	if in.Audience == "users" {
		if len(in.UserIDs)+len(in.Emails) == 0 {
			fields["user_ids"] = "Choose at least one recipient."
		}
		if len(in.UserIDs)+len(in.Emails) > s.maxTargets {
			fields["user_ids"] = "Choose at most 1000 recipients."
		}
		for _, raw := range in.UserIDs {
			id, err := uuid.Parse(strings.TrimSpace(raw))
			if err != nil {
				fields["user_ids"] = "One of the user identifiers is invalid."
				break
			}
			targets = append(targets, id)
		}
		if len(in.Emails) > 0 && len(fields) == 0 {
			emails := make([]string, 0, len(in.Emails))
			for _, e := range in.Emails {
				norm, ok := NormalizeEmail(e)
				if !ok {
					fields["emails"] = "One of the email addresses is invalid."
					break
				}
				emails = append(emails, norm)
			}
			if fields["emails"] == "" {
				ids, err := s.repo.UserIDsByEmails(ctx, emails)
				if err != nil {
					return nil, apperror.Internal(err)
				}
				targets = append(targets, ids...)
			}
		}
	} else if len(in.UserIDs) > 0 || len(in.Emails) > 0 {
		fields["user_ids"] = "Recipients can only be chosen with the users audience."
	}
	if in.CampaignID != nil {
		if _, err := s.growth.GetCampaign(ctx, *in.CampaignID); err != nil {
			fields["campaign_id"] = "Unknown campaign."
		}
	}
	if len(fields) > 0 {
		return nil, apperror.Validation(fields)
	}
	if in.Audience == "users" && len(targets) == 0 {
		return nil, apperror.Validation(map[string]string{"user_ids": "None of the recipients match an account."})
	}
	ids := make(domain.StringList, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, t.String())
	}
	now := s.clock.Now()
	b := &domain.Broadcast{ID: domain.NewID(), CreatedBy: &adminID, Audience: in.Audience, OnlineOnly: in.OnlineOnly, TargetIDs: ids,
		CampaignID: in.CampaignID, Title: title, Body: body, Link: link, Status: "queued", CreatedAt: now}
	if err := s.repo.CreateBroadcast(ctx, b); err != nil {
		return nil, apperror.Internal(err)
	}
	recordAudit(ctx, s.audit, s.clock, s.ipSecret, "broadcast.created", "broadcast", b.ID.String(), map[string]any{
		"audience": b.Audience, "online_only": b.OnlineOnly, "targets": len(targets), "campaign_id": uuidString(b.CampaignID)})
	// Delivery runs detached from the request: the admin gets 202 at once.
	bg := database.Detach(context.WithoutCancel(ctx))
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.deliverBroadcast(bg, b, targets)
	}()
	v := broadcastView(b)
	return &v, nil
}

func uuidString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

// Wait blocks until in-flight broadcasts are delivered (shutdown, tests).
func (s *NotificationService) Wait() { s.wg.Wait() }

const broadcastPage = 5000

func (s *NotificationService) deliverBroadcast(ctx context.Context, b *domain.Broadcast, targets []uuid.UUID) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	log := s.log.With(zap.String("broadcast_id", b.ID.String()))
	_ = s.repo.UpdateBroadcast(ctx, b.ID, map[string]any{"status": "sending"})

	var roles []domain.Role
	switch b.Audience {
	case "clients":
		roles = []domain.Role{domain.RoleClient}
	case "psychics":
		roles = []domain.Role{domain.RolePsychic}
	case "all":
		roles = []domain.Role{domain.RoleClient, domain.RolePsychic}
	}
	var ids []uuid.UUID
	if b.OnlineOnly {
		online, err := s.presence.OnlineUsers(ctx, 100000)
		if err != nil {
			log.Error("broadcast: presence lookup failed", zap.Error(err))
			_ = s.repo.UpdateBroadcast(ctx, b.ID, map[string]any{"status": "failed"})
			return
		}
		if b.Audience == "users" {
			online = intersect(online, targets)
		}
		if ids, _ = s.repo.AudienceIDs(ctx, roles, online, b.CampaignID, 100000); ids == nil {
			ids = []uuid.UUID{}
		}
	} else {
		var filter []uuid.UUID
		if b.Audience == "users" {
			filter = targets
		}
		var err error
		if ids, err = s.repo.AudienceIDs(ctx, roles, filter, b.CampaignID, 1000000); err != nil {
			log.Error("broadcast: audience lookup failed", zap.Error(err))
			_ = s.repo.UpdateBroadcast(ctx, b.ID, map[string]any{"status": "failed"})
			return
		}
	}

	data := map[string]any{"broadcast_id": b.ID, "title": map[string]any(b.Title), "body": map[string]any(b.Body)}
	sent := 0
	for start := 0; start < len(ids); start += broadcastPage {
		end := min(start+broadcastPage, len(ids))
		rows := make([]domain.Notification, 0, end-start)
		now := s.clock.Now()
		for _, uid := range ids[start:end] {
			rows = append(rows, domain.Notification{ID: domain.NewID(), UserID: uid, Kind: NotifyBroadcast, Data: domain.JSONMap(data),
				Link: b.Link, BroadcastID: &b.ID, CreatedAt: now})
		}
		if err := s.repo.InsertBatch(ctx, rows); err != nil {
			log.Error("broadcast: insert failed", zap.Error(err))
			_ = s.repo.UpdateBroadcast(ctx, b.ID, map[string]any{"status": "failed", "recipients": sent})
			return
		}
		for i := range rows {
			v := notificationView(&rows[i])
			warnErr(ctx, "broadcast: publish", s.events.Publish(ctx, EventNotificationCreated, map[string]any{"notification": v}, rows[i].UserID))
		}
		sent += len(rows)
	}
	sentAt := s.clock.Now()
	if err := s.repo.UpdateBroadcast(ctx, b.ID, map[string]any{"status": "sent", "recipients": sent, "sent_at": sentAt}); err != nil {
		log.Error("broadcast: status update failed", zap.Error(err))
	}
	log.Info("broadcast delivered", zap.Int("recipients", sent))
}

func intersect(a, b []uuid.UUID) []uuid.UUID {
	set := make(map[uuid.UUID]bool, len(b))
	for _, id := range b {
		set[id] = true
	}
	out := []uuid.UUID{}
	for _, id := range a {
		if set[id] {
			out = append(out, id)
		}
	}
	return out
}

// Broadcasts lists past broadcasts.
func (s *NotificationService) Broadcasts(ctx context.Context, cursor string, limit int) (Page[BroadcastView], error) {
	c, err := decodeCursor(cursor)
	if err != nil {
		return Page[BroadcastView]{}, err
	}
	rows, err := s.repo.ListBroadcasts(ctx, c, limit)
	if err != nil {
		return Page[BroadcastView]{}, apperror.Internal(err)
	}
	page := Page[BroadcastView]{Items: make([]BroadcastView, 0, len(rows))}
	for i := range rows {
		page.Items = append(page.Items, broadcastView(&rows[i]))
	}
	if len(rows) > 0 && len(rows) == clampPage(limit, 20, 100) {
		last := rows[len(rows)-1]
		page.NextCursor = repository.EncodeCursor(last.CreatedAt, last.ID.String())
	}
	return page, nil
}

// GetBroadcast returns one broadcast.
func (s *NotificationService) GetBroadcast(ctx context.Context, id uuid.UUID) (*BroadcastView, error) {
	b, err := s.repo.GetBroadcast(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err, "Broadcast")
	}
	v := broadcastView(b)
	return &v, nil
}

// recordAudit writes an admin audit entry; failures are logged, not fatal.
func recordAudit(ctx context.Context, audit repository.AuditRepository, clk clock.Clock, ipSecret, action, targetType, targetID string, meta map[string]any) {
	p, _ := reqctx.PrincipalFrom(ctx)
	m := reqctx.MetaFrom(ctx)
	var actor *uuid.UUID
	if p.UserID != uuid.Nil {
		id := p.UserID
		actor = &id
	}
	if meta == nil {
		meta = map[string]any{}
	}
	if err := audit.Create(ctx, &domain.AuditLog{ID: domain.NewID(), ActorID: actor, Action: action, TargetType: targetType,
		TargetID: targetID, Metadata: domain.JSONMap(meta), IPHash: securityKeyed(ipSecret, m.IP), RequestID: m.RequestID,
		CreatedAt: clk.Now()}); err != nil {
		warnErr(ctx, "audit write failed", err, zap.String("action", action))
	}
}
