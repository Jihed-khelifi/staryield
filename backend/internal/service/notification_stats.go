package service

import (
	"context"

	"go.uber.org/zap"
)

// NotificationStats gathers the counters of the notification pipeline.
type NotificationStats struct {
	Dispatcher DispatcherStats `json:"dispatcher"`
	Push       PushStats       `json:"push"`
	Broadcasts BroadcastStats  `json:"broadcasts"`
}

// NotificationObserver reports the pipeline counters. The platform has no
// admin system or health surface for them, so Tick logs a summary line once a
// minute, and only when something notable happened since the last tick: a
// drop, a synchronous fallback, a prune, a retry, a failed publish or an
// expired push.
type NotificationObserver struct {
	dispatcher *Dispatcher
	push       *PushService
	broadcasts *NotificationService
	log        *zap.Logger
	last       NotificationStats
}

// NewNotificationObserver wires the observer; any source may be nil.
func NewNotificationObserver(d *Dispatcher, p *PushService, b *NotificationService, log *zap.Logger) *NotificationObserver {
	return &NotificationObserver{dispatcher: d, push: p, broadcasts: b, log: log}
}

// Stats returns the current counters.
func (o *NotificationObserver) Stats() NotificationStats {
	var st NotificationStats
	if o.dispatcher != nil {
		st.Dispatcher = o.dispatcher.Snapshot()
	}
	if o.push != nil {
		st.Push = o.push.Snapshot()
	}
	if o.broadcasts != nil {
		st.Broadcasts = o.broadcasts.BroadcastSnapshot()
	}
	return st
}

// Tick logs a summary when a notable counter moved since the previous tick.
func (o *NotificationObserver) Tick(context.Context) error {
	cur, prev := o.Stats(), o.last
	o.last = cur
	notable := cur.Dispatcher.Dropped != prev.Dispatcher.Dropped || cur.Dispatcher.SyncFallbacks != prev.Dispatcher.SyncFallbacks ||
		cur.Dispatcher.RowFallbacks != prev.Dispatcher.RowFallbacks || cur.Dispatcher.PublishFailed != prev.Dispatcher.PublishFailed ||
		cur.Push.Dropped != prev.Push.Dropped || cur.Push.Pruned != prev.Push.Pruned || cur.Push.Expired != prev.Push.Expired ||
		cur.Push.GaveUp != prev.Push.GaveUp || cur.Broadcasts.Resumed != prev.Broadcasts.Resumed
	if !notable {
		return nil
	}
	o.log.Warn("notification pipeline summary (since start)",
		zap.Int64("written", cur.Dispatcher.Written), zap.Int64("sync_fallbacks", cur.Dispatcher.SyncFallbacks),
		zap.Int64("insert_retries", cur.Dispatcher.InsertRetries), zap.Int64("row_fallbacks", cur.Dispatcher.RowFallbacks),
		zap.Int64("dropped", cur.Dispatcher.Dropped), zap.Int64("publish_retried", cur.Dispatcher.PublishRetried),
		zap.Int64("publish_failed", cur.Dispatcher.PublishFailed), zap.Int("buffered", cur.Dispatcher.Buffered),
		zap.Int64("push_sent", cur.Push.Sent), zap.Int64("push_retried", cur.Push.Retried), zap.Int64("push_expired", cur.Push.Expired),
		zap.Int64("push_pruned", cur.Push.Pruned), zap.Int64("push_dropped", cur.Push.Dropped), zap.Int64("push_rejected", cur.Push.Rejected),
		zap.Int64("push_gave_up", cur.Push.GaveUp), zap.Any("push_depth", cur.Push.Depth),
		zap.Int64("broadcast_chunks", cur.Broadcasts.Chunks), zap.Int64("broadcasts_resumed", cur.Broadcasts.Resumed))
	return nil
}
