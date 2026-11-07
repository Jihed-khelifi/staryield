/**
 * Wire types mirroring the Go API views (backend/internal/service). The
 * server is authoritative for every field; the client only renders them.
 */

export type Role = "client" | "psychic" | "admin";
export type PresenceStatus = "online" | "busy" | "away" | "offline";
export type Availability = "online" | "away" | "offline";

export type Wallet = { balance: number; held: number; free_minutes: number };

export type Profile = {
  nickname: string;
  birth_date: string;
  birth_time: string;
  birth_place_id: string;
  birth_place: string;
  birth_latitude: number | null;
  birth_longitude: number | null;
  birth_timezone: string;
  location: string;
  mood: string;
  theme: string;
  avatar_url: string;
  notify_horoscope: boolean;
  notify_events: boolean;
  notify_chart: boolean;
};

export type PsychicSelf = {
  slug: string;
  name: string;
  image_url: string;
  availability: Availability;
  rate_per_minute: number;
  listed: boolean;
};

export type User = {
  id: string;
  email: string;
  role: Role;
  status: string;
  display_name: string;
  locale: string;
  created_at: string;
  profile: Profile;
  wallet?: Wallet;
  psychic?: PsychicSelf;
};

export type SessionResponse = {
  user: User;
  csrf_token: string;
  access_expires_at: string;
};

export type Service = {
  id: string;
  name: string;
  subtitle: string;
  description: string;
  price_credits: number;
  active: boolean;
  /** Admin-curated ("standard") or the advisor's own offering ("custom"). */
  kind?: "standard" | "custom";
  /** Typical delivery time in days. */
  delivery_days?: number;
};

export type Psychic = {
  id: string;
  slug: string;
  name: string;
  headline: string;
  about: string[];
  philosophy: string;
  image_url: string;
  zodiac: string;
  skills: string[];
  rating: number;
  reviews_count: number;
  readings_count: number;
  years_experience: number;
  rate_per_minute: number;
  status: PresenceStatus;
  listed: boolean;
  featured?: boolean;
  services: Service[];
};

/** An advisor as shown on directory and home cards (GET /psychics). */
export type AdvisorCard = Pick<
  Psychic,
  | "slug"
  | "name"
  | "headline"
  | "image_url"
  | "zodiac"
  | "skills"
  | "rating"
  | "reviews_count"
  | "readings_count"
  | "years_experience"
  | "rate_per_minute"
  | "status"
> & { featured: boolean };

/** One numbered page of the advisor directory. */
export type DirectoryPage = {
  items: AdvisorCard[];
  page: number;
  page_size: number;
  total: number;
  total_pages: number;
};

export type DirectoryQuery = {
  page?: number;
  page_size?: number;
  status?: PresenceStatus;
  q?: string;
  skill?: string;
};

/** A home-page advisor card: "chat" when the advisor can talk now. */
export type FeaturedAdvisor = AdvisorCard & { cta: "chat" | "waitlist" };

export type FeaturedList = { items: FeaturedAdvisor[]; generated_at: string };

/** An advisor the signed-in client is waiting for (told once when online). */
export type WaitlistEntry = {
  slug: string;
  name: string;
  image_url: string;
  status: PresenceStatus;
  joined_at: string;
};

export type CreditPack = {
  id: string;
  name: string;
  credits: number;
  price_cents: number;
  currency: string;
  featured: boolean;
  active: boolean;
  sort_order: number;
};

export type Purchase = {
  id: string;
  pack_id: string;
  credits: number;
  price_cents: number;
  currency: string;
  status: "pending" | "succeeded" | "failed" | "refunded";
  provider: string;
  failure_reason?: string;
  created_at: string;
  completed_at?: string;
  wallet?: Wallet;
};

export type Participant = {
  id: string;
  role: Role;
  name: string;
  avatar_url?: string;
  slug?: string;
  status?: PresenceStatus;
};

export type SessionStatus =
  | "requested"
  | "accepted"
  | "active"
  | "disconnected"
  | "ended"
  | "settled"
  | "declined"
  | "expired"
  | "cancelled";

export type ChatSession = {
  id: string;
  conversation_id: string;
  status: SessionStatus;
  end_reason?: string;
  client: Participant;
  psychic: Participant;
  rate_per_minute: number;
  requested_at: string;
  expires_at?: string;
  accepted_at?: string;
  started_at?: string;
  disconnected_at?: string;
  disconnected_party?: string;
  grace_expires_at?: string;
  ended_at?: string;
  settled_at?: string;
  next_billing_at?: string;
  active_seconds: number;
  billed_minutes: number;
  free_minutes_used: number;
  credits_charged?: number;
  psychic_earnings?: number;
  // --- FE-CHAT (wave 3): member discount (client) and live earnings (advisor) ---
  /** The advisor's list rate; above rate_per_minute when a member discount applies. */
  list_rate_per_minute?: number;
  /** Client viewer: member discount in percent (absent without one). */
  discount_pct?: number;
  /** Advisor viewer: psychic_earnings in money (minor units of `currency`). */
  psychic_earnings_cents?: number;
  currency?: string;
  // --- end FE-CHAT ---
  // --- revisions 3.0: free mode (§15.4) ---
  /** Requested while free mode was on: the platform pays every minute. */
  sponsored?: boolean;
  sponsored_minutes?: number;
  sponsored_credits?: number;
  reconnects: number;
  server_time: string;
};

export type SessionSummary = {
  session: ChatSession;
  duration_seconds: number;
  messages: number;
  /** Client viewer: the session can still be rated (§9.2). */
  can_review?: boolean;
  /** Client viewer: the review already left for this session. */
  review?: Review;
};

/** Image attachment metadata (§9.1); bytes are fetched with credentials. */
export type ChatAttachment = {
  id: string;
  content_type: string;
  width: number;
  height: number;
  bytes: number;
};

export type ChatMessage = {
  id: string;
  conversation_id: string;
  session_id: string;
  sender_id?: string;
  kind: "text" | "system" | "image";
  /** Text, or the (possibly empty) caption of an image message. */
  body: string;
  attachments?: ChatAttachment[];
  client_msg_id?: string;
  created_at: string;
  saved?: boolean;
  /** FE-CHAT (wave 3): position in the conversation (1, 2, 3, … without gaps). */
  seq?: number;
};

export type Conversation = {
  id: string;
  counterpart: Participant;
  last_message?: ChatMessage;
  last_message_at?: string;
  unread: number;
  open_session?: ChatSession;
  created_at: string;
  // --- FE-CHAT (wave 3): seq read positions ---
  last_activity_at?: string;
  last_seq?: number;
  /** How far the viewer has read. */
  read_seq?: number;
  /** How far the other participant has read ("Seen"). */
  peer_read_seq?: number;
  // --- end FE-CHAT ---
};

export type PsychicThread = {
  psychic: Psychic;
  conversation?: Conversation;
  open_session?: ChatSession;
  client_open_session?: ChatSession;
};

export type Page<T> = { items: T[]; next_cursor?: string };

// --- FE-CHAT (wave 3): chat paging contracts (backend chat_views.go) ---
/** One inbox page, newest activity first. */
export type InboxPage = { items: Conversation[]; next_cursor?: string; has_more: boolean };
/** One page of history in chronological order; older pages via before_seq=next_before_seq. */
export type MessagePage = { items: ChatMessage[]; has_more: boolean; next_before_seq?: number; next_cursor?: string };
/** Messages after a seq, oldest first (gap filling); last_seq is the conversation head. */
export type SincePage = { items: ChatMessage[]; has_more: boolean; last_seq: number };
/** The caller's read state after moving the read position. */
export type ReadResult = { conversation_id: string; read_seq: number; unread: number };
/** `session.billed`: per started minute, to each participant. */
export type SessionBilledEvent = {
  session_id: string;
  minute: number;
  free: boolean;
  /** Free mode (§15.4): nothing was taken from the wallet. */
  sponsored?: boolean;
  /** Client: credits paid; advisor: the list rate (0 for free minutes). */
  charged: number;
  next_billing_at: string;
  active_seconds: number;
  /** Client only. */
  minutes_remaining?: number;
  wallet?: Wallet;
  /** Advisor only: live earnings so far. */
  psychic_earnings?: number;
  psychic_earnings_cents?: number;
  currency?: string;
};
/** `message.read`: the other participant read up to read_seq. */
export type MessageReadEvent = { conversation_id: string; reader_id: string; read_seq?: number; message_id?: string; read_at?: string };
// --- end FE-CHAT ---

export type LedgerEntry = {
  id: string;
  type: string;
  amount: number;
  held_amount: number;
  balance_after: number;
  held_after: number;
  reference_type?: string;
  reference_id?: string;
  description: string;
  created_at: string;
};

export type CustomRequestStatus =
  | "pending"
  | "accepted"
  | "delivered"
  | "declined"
  | "cancelled"
  | "expired"
  | "disputed"
  | "refunded";

export type CustomRequest = {
  id: string;
  client: Participant;
  psychic: Participant;
  service_id: string;
  service_name: string;
  price_credits: number;
  status: CustomRequestStatus;
  client_note: string;
  preferred_at?: string;
  delivery?: string;
  decline_reason?: string;
  dispute_reason?: string;
  resolution_note?: string;
  accept_by: string;
  deliver_by?: string;
  accepted_at?: string;
  delivered_at?: string;
  closed_at?: string;
  created_at: string;
  wallet?: Wallet;
};

export type EarningsReport = {
  summary: {
    gross_credits: number;
    commission_credits: number;
    net_credits: number;
    sessions: number;
    custom_requests: number;
  };
  entries: Page<{
    id: string;
    source_type: string;
    source_id: string;
    kind: string;
    gross_credits: number;
    net_credits: number;
    created_at: string;
  }>;
};

export type ChartPosition = {
  body: string;
  longitude: number;
  sign: string;
  degree: number;
  minute: number;
  second: number;
  formatted: string;
  house?: number;
  house_label?: string;
  retrograde: boolean;
};

export type Chart = {
  instant: string;
  has_location: boolean;
  house_system?: string;
  sun_sign: string;
  moon_sign: string;
  rising_sign?: string;
  positions: ChartPosition[];
  elements: Record<string, number>;
  birth_date: string;
  birth_time?: string;
  time_known: boolean;
  life_path: number;
  disclaimer: string;
};

export type Horoscope = {
  sign: string;
  date: string;
  text: string;
  focus: string;
  affirmation: string;
  mood: string;
  lucky_number: number;
  compatibility: number;
  compatibility_label: string;
  moon_sign: string;
  moon_phase: string;
};

export type CosmicEvents = {
  lunar: { kind: string; at: string; sign: string }[];
  retrogrades: { body: string; start: string; end: string }[];
};

export type Place = {
  id: string;
  name: string;
  country_code: string;
  country: string;
  latitude: number;
  longitude: number;
  time_zone: string;
};

// --- Platform extensions (docs/platform-extensions.md) ----------------------

/** First-touch campaign attribution sent on sign-up (§5.4). */
export type AttributionInput = {
  click_id?: string;
  utm_source?: string;
  utm_medium?: string;
  utm_campaign?: string;
  utm_content?: string;
  utm_term?: string;
  referrer?: string;
  landing_path?: string;
  first_seen_at?: string;
  /** Analytics visitor id (localStorage `staryield.aid`), only when analytics is allowed (§17). */
  anonymous_id?: string;
};

export type NotificationKind =
  | "chat.requested"
  | "chat.missed"
  | "chat.review_prompt"
  | "review.received"
  | "request.created"
  | "request.updated"
  | "purchase.succeeded"
  | "credits.adjusted"
  | "withdrawal.updated"
  | "photo.approved"
  | "photo.rejected"
  | "support.replied"
  | "broadcast";

/** An in-app notification (§7.1). Named to avoid the DOM `Notification`. */
export type AppNotification = {
  id: string;
  kind: NotificationKind | (string & {});
  data: Record<string, unknown>;
  /** Locale-less app path ("/profile"); the client prefixes the locale. */
  link: string;
  read_at?: string;
  created_at: string;
};

/** A client's rating of a session (§9.2). */
export type Review = {
  id: string;
  session_id?: string;
  rating: number;
  comment: string;
  status?: "visible" | "hidden";
  author: { name: string; avatar_url?: string };
  created_at: string;
};

/** An advisor's own note about a client (§9.5). */
export type Note = {
  id: string;
  body: string;
  shared: boolean;
  created_at: string;
  updated_at: string;
};

/** A note another advisor shared about the same client (read-only). */
export type SharedNote = {
  id: string;
  body: string;
  author: { name: string; slug: string; avatar_url?: string };
  created_at: string;
  updated_at: string;
};

/** A KPI share; `pct` is null when nobody qualified yet (§8.1). */
export type StatsRatio = { pct: number | null; numerator: number; denominator: number };

export type StatsKPIs = {
  free_to_paid: StatsRatio;
  paid_to_retained: StatsRatio;
  paid_minutes: number;
  retained_repeat_days: { median_days: number | null; sample: number };
  active_hours: { hours: number; in_session_hours: number; tracked_since?: string };
};

/** "Advisor stats overview" cards for a window and the window before it. */
export type StatsOverview = {
  range: { from: string; to: string; tz: string };
  kpis: StatsKPIs;
  previous: StatsKPIs;
};

/** Withdrawable earnings for the advisor's header wallet pill. */
export type PsychicBalance = {
  available_credits: number;
  available_cents: number;
  currency: string;
  reserved_credits: number;
  lifetime_net_credits: number;
};

/** What an advisor sees about the client of a conversation (§9.3). */
export type ClientProfile = {
  id: string;
  name: string;
  avatar_url?: string;
  online: boolean;
  locale: string;
  country_code?: string;
  age?: number;
  member_since: string;
  astro?: {
    sun_sign: string;
    sun_sign_uncertain?: boolean;
    moon_sign?: string;
    rising_sign?: string;
    life_path?: number;
    time_known: boolean;
    /** Natal planets; degree and house only when the birth time is known. */
    placements?: { body: string; sign: string; degree?: number; house?: number; retrograde: boolean }[];
  };
  /** Whether the client chose to share their date and place of birth. */
  birth_details_shared: boolean;
  /** Only present when shared: never the birth time or coordinates. */
  birth_details?: { date_of_birth: string; place?: string };
  horoscope?: { sign: string; date: string; text: string; focus: string; mood: string; affirmation: string };
  history: { sessions: number; minutes: number; first_session_at?: string; last_session_at?: string };
};

export type AnalyticsInterval = "hour" | "day" | "week" | "month";
export type HourLevel = "peak" | "medium" | "low" | "none";
export type ScoreGrade = "excellent" | "good" | "fair" | "needs_attention";

/** Advisor performance dashboard (§8). */
export type PsychicAnalytics = {
  range: { from: string; to: string; tz: string; interval: string };
  earnings: {
    gross_credits: number;
    commission_credits: number;
    net_credits: number;
    sessions: number;
    custom_requests: number;
    series: { bucket: string; net_credits: number; gross_credits: number; sessions: number }[];
  };
  payout: {
    available_credits: number;
    reserved_credits: number;
    paid_credits: number;
    lifetime_net_credits: number;
    cents_per_credit: number;
    currency: string;
    min_withdrawal_credits: number;
  };
  sessions: {
    requested: number;
    accepted: number;
    declined: number;
    expired: number;
    cancelled: number;
    completed: number;
    acceptance_rate: number;
    avg_response_seconds: number;
    avg_duration_seconds: number;
    total_minutes: number;
  };
  clients: {
    unique: number;
    new: number;
    returning: number;
    returning_rate: number;
    top: { name: string; sessions: number; minutes: number }[];
  };
  rating: { average: number; count: number; distribution: Record<"1" | "2" | "3" | "4" | "5", number> };
  score: {
    value: number;
    grade: ScoreGrade;
    components: { acceptance: number; rating: number; retention: number; completion: number; responsiveness: number };
  };
  hours: {
    grid: number[][];
    levels: HourLevel[][];
    best: { weekday: number; hour: number; sessions: number }[];
  };
};

export type WithdrawalMethod = "bank_transfer" | "paypal";
export type WithdrawalStatus = "pending" | "approved" | "paid" | "rejected" | "cancelled";

export type Withdrawal = {
  id: string;
  amount_credits: number;
  amount_cents: number;
  currency: string;
  method: WithdrawalMethod;
  destination_hint: string;
  status: WithdrawalStatus;
  note: string;
  requested_at: string;
  processed_at?: string;
};

// --- Support (contact form) ------------------------------------------------------

/** Contact-form topic; mirrors the API's allow-list. */
export type SupportTopic = "general" | "privacy" | "billing" | "advisor" | "technical";

/** POST /api/v1/support/tickets body. */
export type TicketInput = {
  email: string;
  topic: SupportTopic;
  subject: string;
  message: string;
  consent: boolean;
  name?: string;
};

/** What the sender gets back: a short reference such as SY-7K2M9Q. */
export type TicketReceipt = { reference: string; topic: SupportTopic; created_at: string };

// --- Advisor settings (§8.2) --------------------------------------------------------

/** GET/PATCH /psychic/settings: the advisor's own settings (phone shown to its owner only). */
export type AdvisorSettings = {
  /** Increments on every save; send it back as `expected_version`. */
  version: number;
  /** E.164 ("+14155552671") or "". */
  phone: string;
  /** "+•••••••••71" or "". */
  phone_masked: string;
  headline: string;
  about: string[];
  philosophy: string;
  /** The public profile's skills. */
  modalities: string[];
};

export type AdvisorSettingsPatch = Partial<Pick<AdvisorSettings, "phone" | "headline" | "about" | "philosophy" | "modalities">> & {
  expected_version?: number;
};

export type PhotoSubmissionStatus = "pending" | "approved" | "rejected";

/** A profile photo the advisor submitted; private until an administrator approves it. */
export type PhotoSubmission = {
  id: string;
  status: PhotoSubmissionStatus;
  /** Why it was declined (rejected only). */
  reason?: string;
  width: number;
  height: number;
  bytes: number;
  /** The private image can still be previewed (GET /psychic/photo/image). */
  has_image: boolean;
  submitted_at: string;
  reviewed_at?: string;
};

export type PhotoLimits = { max_bytes: number; max_pixels: number; min_side: number; daily_uploads: number };

/** GET /psychic/photo: the live public photo and the latest submission. */
export type PhotoState = { image_url: string; submission: PhotoSubmission | null; limits: PhotoLimits };

/** The advisor's own custom-request offering ("Customs"). */
export type CustomService = {
  id: string;
  name: string;
  description: string;
  price_credits: number;
  delivery_days: number;
  /** False when the Staryield team has hidden it. */
  active: boolean;
  created_at: string;
  updated_at: string;
};

export type CustomServiceLimits = {
  name_max: number;
  description_max: number;
  min_price_credits: number;
  max_price_credits: number;
  min_delivery_days: number;
  max_delivery_days: number;
};

export type CustomServiceState = { service: CustomService | null; limits: CustomServiceLimits };

export type CustomServiceInput = { name: string; price_credits: number; delivery_days: number; description?: string };

// --- Revisions 3.0 (docs/platform-extensions.md §15–§18) ---------------------------

/** One promo banner of the public site (§15.1), already localized. */
export type SiteBanner = {
  id: string;
  text: string;
  /** "" when the banner has no call to action label. */
  cta_label: string;
  /** href: locale-less path ("/signup") or an allow-listed https URL; "" for "none". */
  action: { type: "link" | "none"; href: string };
  audience: "all" | "anonymous" | "signed_in";
};

/** GET /site/config: public runtime settings (§15.1). */
export type SiteConfig = {
  banners: { rotation_seconds: number; items: SiteBanner[] };
  signup_offer: { enabled: boolean; free_minutes: number };
  free_mode: { active: boolean; ends_at?: string };
  generated_at: string;
};

/** GET /chat/unread-count and the `chat.unread` realtime event (§18). */
export type ChatUnread = {
  count: number;
  conversations: number;
  conversation_id?: string;
  conversation_unread?: number;
};

export type TicketStatus = "open" | "pending" | "resolved" | "closed";

/** A requester's support request (§16.2). */
export type CustomerTicketRow = {
  reference: string;
  subject: string;
  topic: SupportTopic;
  status: TicketStatus;
  /** open → support, pending → you, resolved/closed → none. */
  awaiting: "support" | "you" | "none";
  created_at: string;
  last_message_at: string;
};

export type CustomerTicketMessage = {
  id: string;
  author: { type: "customer" | "staff"; name: string };
  body: string;
  created_at: string;
};

export type CustomerTicket = CustomerTicketRow & {
  messages: CustomerTicketMessage[];
  /** False once the request is closed. */
  can_reply: boolean;
};
