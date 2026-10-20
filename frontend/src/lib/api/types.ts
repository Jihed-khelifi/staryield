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
  services: Service[];
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
  reconnects: number;
  server_time: string;
};

export type SessionSummary = {
  session: ChatSession;
  duration_seconds: number;
  messages: number;
};

export type ChatMessage = {
  id: string;
  conversation_id: string;
  session_id: string;
  sender_id?: string;
  kind: "text" | "system";
  body: string;
  client_msg_id?: string;
  created_at: string;
  saved?: boolean;
};

export type Conversation = {
  id: string;
  counterpart: Participant;
  last_message?: ChatMessage;
  last_message_at?: string;
  unread: number;
  open_session?: ChatSession;
  created_at: string;
};

export type PsychicThread = {
  psychic: Psychic;
  conversation?: Conversation;
  open_session?: ChatSession;
  client_open_session?: ChatSession;
};

export type Page<T> = { items: T[]; next_cursor?: string };

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
