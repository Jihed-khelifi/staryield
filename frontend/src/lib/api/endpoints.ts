/** Typed wrappers for every API route used by the web app. */
import { api, newId, setAuthHint, setCsrfToken } from "@/lib/api/client";
import type {
  AnalyticsInterval,
  AppNotification,
  AttributionInput,
  Availability,
  ClientProfile,
  Chart,
  ChatMessage,
  ChatSession,
  ChatUnread,
  Conversation,
  CustomerTicket,
  CustomerTicketRow,
  SiteConfig,
  InboxPage,
  MessagePage,
  ReadResult,
  SincePage,
  CosmicEvents,
  CreditPack,
  CustomRequest,
  CustomRequestStatus,
  DirectoryPage,
  DirectoryQuery,
  EarningsReport,
  FeaturedList,
  Horoscope,
  LedgerEntry,
  Note,
  Page,
  Place,
  Profile,
  Psychic,
  PsychicThread,
  Purchase,
  PsychicAnalytics,
  PsychicBalance,
  Review,
  SessionResponse,
  SessionSummary,
  SharedNote,
  StatsOverview,
  User,
  WaitlistEntry,
  Wallet,
  Withdrawal,
  WithdrawalMethod,
  TicketInput,
  TicketReceipt,
  AdvisorSettings,
  AdvisorSettingsPatch,
  CustomServiceInput,
  CustomServiceState,
  PhotoState,
} from "@/lib/api/types";

function rememberSession(res: SessionResponse): SessionResponse {
  setCsrfToken(res.csrf_token);
  setAuthHint(true);
  return res;
}

export const authApi = {
  register: (input: {
    email: string;
    password: string;
    accept_terms: boolean;
    birth_date?: string;
    display_name?: string;
    locale?: string;
    attribution?: AttributionInput;
  }) =>
    api<SessionResponse>("/api/v1/auth/register", { method: "POST", body: input, skipRefresh: true }).then(rememberSession),
  login: (email: string, password: string) =>
    api<SessionResponse>("/api/v1/auth/login", { method: "POST", body: { email, password }, skipRefresh: true }).then(rememberSession),
  logout: () =>
    api<void>("/api/v1/auth/logout", { method: "POST", skipRefresh: true }).finally(() => setAuthHint(false)),
  logoutAll: () => api<void>("/api/v1/auth/logout-all", { method: "POST" }).finally(() => setAuthHint(false)),
  forgotPassword: (email: string, locale: string) =>
    api<{ status: string }>("/api/v1/auth/password/forgot", { method: "POST", body: { email, locale }, skipRefresh: true }),
  resetPassword: (token: string, password: string) =>
    api<void>("/api/v1/auth/password/reset", { method: "POST", body: { token, password }, skipRefresh: true }),
  changePassword: (current_password: string, new_password: string) =>
    api<SessionResponse>("/api/v1/auth/password/change", { method: "POST", body: { current_password, new_password } }).then(rememberSession),
};

export type ProfileUpdate = Partial<Pick<Profile, "nickname" | "birth_date" | "birth_time" | "birth_place_id" | "location" | "mood" | "theme" | "notify_horoscope" | "notify_events" | "notify_chart">> & {
  display_name?: string;
  locale?: string;
};

export const meApi = {
  get: (signal?: AbortSignal) => api<User>("/api/v1/me", { signal }),
  update: (input: ProfileUpdate) => api<User>("/api/v1/me/profile", { method: "PATCH", body: input }),
  uploadAvatar: (file: File) => {
    const form = new FormData();
    form.append("avatar", file);
    return api<User>("/api/v1/me/avatar", { method: "POST", body: form, timeoutMs: 30_000 });
  },
  deleteAvatar: () => api<User>("/api/v1/me/avatar", { method: "DELETE" }),
  chart: (signal?: AbortSignal) => api<Chart>("/api/v1/me/chart", { signal }),
  horoscope: (date?: string, signal?: AbortSignal) => api<Horoscope>("/api/v1/me/horoscope", { query: { date }, signal }),
};

export const walletApi = {
  get: () => api<Wallet>("/api/v1/wallet"),
  ledger: (cursor?: string) => api<Page<LedgerEntry>>("/api/v1/wallet/ledger", { query: { cursor } }),
  packs: (signal?: AbortSignal) => api<{ items: CreditPack[] }>("/api/v1/credit-packs", { signal }).then((r) => r.items),
  purchase: (pack_id: string, idempotency_key: string, receipt_email?: string) =>
    api<Purchase>("/api/v1/credits/purchases", { method: "POST", body: { pack_id, idempotency_key, receipt_email }, idempotent: true, timeoutMs: 30_000 }),
  purchases: () => api<Page<Purchase>>("/api/v1/credits/purchases"),
};

export const waitlistApi = {
  /** Joins an advisor's waiting list (idempotent; 409 advisor_available when online). */
  join: (slug: string) => api<WaitlistEntry>(`/api/v1/psychics/${encodeURIComponent(slug)}/waitlist`, { method: "POST" }),
  leave: (slug: string) => api<void>(`/api/v1/psychics/${encodeURIComponent(slug)}/waitlist`, { method: "DELETE" }),
  mine: (params: { cursor?: string; limit?: number } = {}, signal?: AbortSignal) =>
    api<Page<WaitlistEntry>>("/api/v1/me/waitlist", { query: params, signal }),
};

export const psychicsApi = {
  /** Numbered directory page; filters are optional. */
  browse: (query: DirectoryQuery = {}, signal?: AbortSignal) => api<DirectoryPage>("/api/v1/psychics", { query, signal }),
  /** Home-page selection: available advisors first, then best rated (1–6). */
  featured: (limit?: number, signal?: AbortSignal) => api<FeaturedList>("/api/v1/psychics/featured", { query: { limit }, signal }),
  get: (slug: string, signal?: AbortSignal) => api<Psychic>(`/api/v1/psychics/${encodeURIComponent(slug)}`, { signal }),
  setAvailability: (availability: Availability) => api<Psychic>("/api/v1/psychic/availability", { method: "PATCH", body: { availability } }),
  earnings: () => api<EarningsReport>("/api/v1/psychic/earnings"),
  /** Withdrawable earnings for the header wallet pill. */
  balance: (signal?: AbortSignal) => api<PsychicBalance>("/api/v1/psychic/balance", { signal }),
  /** "Advisor stats overview" KPIs; dates are calendar days in `tz`. */
  stats: (params: { from?: string; to?: string; tz?: string }, signal?: AbortSignal) =>
    api<StatsOverview>("/api/v1/psychic/stats/overview", { query: params, signal }),
  /** Own dashboard (§8). `tz` is the viewer's IANA zone for hour/day buckets. */
  analytics: (params: { from?: string; to?: string; tz?: string; interval?: AnalyticsInterval }, signal?: AbortSignal) =>
    api<PsychicAnalytics>("/api/v1/psychic/analytics", { query: params, signal }),
};

/** Maximum chat image size accepted by the API (§9.1). */
export const ATTACHMENT_MAX_BYTES = 8 * 1024 * 1024;

/** Build the multipart body of an image message (exported for tests). */
export function attachmentForm(file: Blob, caption?: string, clientMsgId?: string, filename = "image"): FormData {
  const form = new FormData();
  form.append("image", file, file instanceof File ? file.name : filename);
  if (caption) form.append("caption", caption);
  if (clientMsgId) form.append("client_msg_id", clientMsgId);
  return form;
}

export const reviewsApi = {
  /** Rate a finished session (client of that session, once, within 7 days). */
  submit: (sessionId: string, rating: number, comment?: string) =>
    api<Review>(`/api/v1/chat/sessions/${encodeURIComponent(sessionId)}/review`, {
      method: "POST",
      body: comment ? { rating, comment } : { rating },
    }),
  /** Public, visible reviews of an advisor. */
  forPsychic: (slug: string, cursor?: string, limit = 10, signal?: AbortSignal) =>
    api<Page<Review>>(`/api/v1/psychics/${encodeURIComponent(slug)}/reviews`, { query: { cursor, limit }, signal }),
  /** The signed-in advisor's own visible reviews. */
  mine: (cursor?: string, signal?: AbortSignal, limit?: number) =>
    api<Page<Review>>("/api/v1/psychic/reviews", { query: { cursor, limit }, signal }),
};

export const withdrawalsApi = {
  list: (cursor?: string, signal?: AbortSignal) => api<Page<Withdrawal>>("/api/v1/psychic/withdrawals", { query: { cursor }, signal }),
  request: (input: { amount_credits: number; method: WithdrawalMethod; destination: string; idempotency_key: string }) =>
    api<Withdrawal>("/api/v1/psychic/withdrawals", { method: "POST", body: input, idempotent: true }),
  cancel: (id: string) => api<Withdrawal>(`/api/v1/psychic/withdrawals/${encodeURIComponent(id)}/cancel`, { method: "POST", idempotent: true }),
};

export const notificationsApi = {
  list: (cursor?: string, limit = 20, signal?: AbortSignal) =>
    api<Page<AppNotification>>("/api/v1/me/notifications", { query: { cursor, limit }, signal }),
  unreadCount: (signal?: AbortSignal) => api<{ count: number }>("/api/v1/me/notifications/unread-count", { signal }),
  markRead: (ids: string[]) => api<{ unread: number }>("/api/v1/me/notifications/read", { method: "POST", body: { ids }, idempotent: true }),
  markAllRead: () => api<{ unread: number }>("/api/v1/me/notifications/read", { method: "POST", body: { all: true }, idempotent: true }),
};

export const chatApi = {
  conversations: (signal?: AbortSignal) => api<{ items: Conversation[] }>("/api/v1/chat/conversations", { signal }).then((r) => r.items),
  thread: (slug: string, signal?: AbortSignal) => api<PsychicThread>(`/api/v1/chat/psychics/${encodeURIComponent(slug)}`, { signal }),
  messages: (conversationId: string, before?: string, signal?: AbortSignal) =>
    api<Page<ChatMessage>>(`/api/v1/chat/conversations/${conversationId}/messages`, { query: { before }, signal }),
  messagesSince: (conversationId: string, after: string) =>
    api<{ items: ChatMessage[] }>(`/api/v1/chat/conversations/${conversationId}/messages/since`, { query: { after } }).then((r) => r.items),
  markRead: (conversationId: string, messageId: string) =>
    api<void>(`/api/v1/chat/conversations/${conversationId}/read`, { method: "POST", body: { message_id: messageId }, idempotent: true }),
  start: (psychic_slug: string, request_key: string = newId()) =>
    api<ChatSession>("/api/v1/chat/sessions", { method: "POST", body: { psychic_slug, request_key }, idempotent: true }),
  sessions: (status?: string, cursor?: string) => api<Page<ChatSession>>("/api/v1/chat/sessions", { query: { status, cursor } }),
  session: (id: string) => api<ChatSession>(`/api/v1/chat/sessions/${id}`),
  /** Unread messages over all of the caller's conversations (§18). */
  unreadCount: (signal?: AbortSignal) => api<ChatUnread>("/api/v1/chat/unread-count", { signal }),
  summary: (id: string) => api<SessionSummary>(`/api/v1/chat/sessions/${id}/summary`),
  accept: (id: string) => api<ChatSession>(`/api/v1/chat/sessions/${id}/accept`, { method: "POST", idempotent: true }),
  decline: (id: string) => api<ChatSession>(`/api/v1/chat/sessions/${id}/decline`, { method: "POST", idempotent: true }),
  cancel: (id: string) => api<ChatSession>(`/api/v1/chat/sessions/${id}/cancel`, { method: "POST", idempotent: true }),
  end: (id: string) => api<SessionSummary>(`/api/v1/chat/sessions/${id}/end`, { method: "POST", idempotent: true }),
  send: (sessionId: string, body: string, client_msg_id: string) =>
    api<{ message: ChatMessage }>(`/api/v1/chat/sessions/${sessionId}/messages`, { method: "POST", body: { body, client_msg_id }, idempotent: true }),
  typing: (sessionId: string, typing: boolean) => api<void>(`/api/v1/chat/sessions/${sessionId}/typing`, { method: "POST", body: { typing } }),
  /** Upload an image message (multipart, §9.1). Resolves with the created message. */
  uploadAttachment: (sessionId: string, file: Blob, caption?: string, clientMsgId?: string) =>
    api<{ message: ChatMessage }>(`/api/v1/chat/sessions/${encodeURIComponent(sessionId)}/attachments`, {
      method: "POST",
      body: attachmentForm(file, caption, clientMsgId),
      timeoutMs: 60_000,
    }),
  /** Image bytes, only for the two participants (cookies + refresh handled by the client). */
  attachment: (id: string, signal?: AbortSignal) =>
    api<Blob>(`/api/v1/chat/attachments/${encodeURIComponent(id)}`, { responseType: "blob", signal, timeoutMs: 30_000 }),
  /** The client's profile for the advisor of a conversation (§9.3). */
  clientProfile: (conversationId: string, signal?: AbortSignal) =>
    api<ClientProfile>(`/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/client`, { signal }),
  saved: (conversationId?: string) =>
    api<{ items: ChatMessage[] }>("/api/v1/chat/saved", { query: { conversation_id: conversationId } }).then((r) => r.items),
  save: (messageId: string) => api<void>(`/api/v1/chat/saved/${messageId}`, { method: "PUT", idempotent: true }),
  unsave: (messageId: string) => api<void>(`/api/v1/chat/saved/${messageId}`, { method: "DELETE", idempotent: true }),
  // --- FE-CHAT (wave 3): seq history, gap filling, inbox and saved pages ---
  /** One inbox page (newest activity first); `next_cursor` only when more exist. */
  inbox: (cursor?: string, limit = 20, signal?: AbortSignal) =>
    api<InboxPage>("/api/v1/chat/conversations", { query: { cursor, limit }, signal }),
  /** History before `beforeSeq` (the newest page when absent), chronological. */
  history: (conversationId: string, beforeSeq?: number, limit = 50, signal?: AbortSignal) =>
    api<MessagePage>(`/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/messages`, {
      query: { before_seq: beforeSeq, limit },
      signal,
    }),
  /** Messages after `afterSeq`, oldest first; follow `has_more` from the last item. */
  since: (conversationId: string, afterSeq: number, limit = 200, signal?: AbortSignal) =>
    api<SincePage>(`/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/messages/since`, {
      query: { after_seq: afterSeq, limit },
      signal,
    }),
  /** Move the caller's read position forward to `seq` (never backwards). */
  markReadSeq: (conversationId: string, seq: number) =>
    api<ReadResult>(`/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/read`, {
      method: "POST",
      body: { seq },
      idempotent: true,
    }),
  /** One page of saved messages (most recently saved first). */
  savedPage: (conversationId: string | undefined, cursor?: string, limit = 5, signal?: AbortSignal) =>
    api<Page<ChatMessage>>("/api/v1/chat/saved", { query: { conversation_id: conversationId, cursor, limit }, signal }),
  // --- end FE-CHAT ---
};

/** Advisors' notes about the client of a conversation (§9.5). */
export const notesApi = {
  list: (conversationId: string, cursor?: string, limit = 5, signal?: AbortSignal) =>
    api<Page<Note>>(`/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/notes`, { query: { cursor, limit }, signal }),
  create: (conversationId: string, input: { body: string; shared: boolean }) =>
    api<Note>(`/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/notes`, { method: "POST", body: input }),
  update: (noteId: string, patch: { body?: string; shared?: boolean }) =>
    api<Note>(`/api/v1/chat/notes/${encodeURIComponent(noteId)}`, { method: "PATCH", body: patch, idempotent: true }),
  remove: (noteId: string) => api<void>(`/api/v1/chat/notes/${encodeURIComponent(noteId)}`, { method: "DELETE", idempotent: true }),
  /** Notes other advisors shared about the same client (read-only). */
  shared: (conversationId: string, cursor?: string, limit = 5, signal?: AbortSignal) =>
    api<Page<SharedNote>>(`/api/v1/chat/conversations/${encodeURIComponent(conversationId)}/notes/shared`, { query: { cursor, limit }, signal }),
};

export const realtimeApi = {
  ticket: () => api<{ ticket: string; expires_in: number; path: string }>("/api/v1/realtime/ticket", { method: "POST", retries: 1, idempotent: true }),
  presence: (connection_id: string, session_id: string, action: "join" | "leave") =>
    api<ChatSession | { left: boolean }>("/api/v1/realtime/presence", { method: "POST", body: { connection_id, session_id, action }, idempotent: true }),
};

export const requestsApi = {
  list: (status?: CustomRequestStatus[], cursor?: string, limit?: number, signal?: AbortSignal) =>
    api<Page<CustomRequest>>("/api/v1/custom-requests", { query: { status: status?.join(","), cursor, limit }, signal }),
  order: (input: { service_id: string; note?: string; preferred_at?: string; idempotency_key: string }) =>
    api<CustomRequest>("/api/v1/custom-requests", { method: "POST", body: input, idempotent: true }),
  cancel: (id: string) => api<CustomRequest>(`/api/v1/custom-requests/${id}/cancel`, { method: "POST", idempotent: true }),
  accept: (id: string) => api<CustomRequest>(`/api/v1/custom-requests/${id}/accept`, { method: "POST", idempotent: true }),
  decline: (id: string, reason?: string) =>
    api<CustomRequest>(`/api/v1/custom-requests/${id}/decline`, { method: "POST", body: { reason: reason ?? "" }, idempotent: true }),
  deliver: (id: string, delivery: string) =>
    api<CustomRequest>(`/api/v1/custom-requests/${id}/deliver`, { method: "POST", body: { delivery }, idempotent: true }),
  dispute: (id: string, reason: string) =>
    api<CustomRequest>(`/api/v1/custom-requests/${id}/dispute`, { method: "POST", body: { reason } }),
};

/** Contact form (public; signed-in senders are linked to their account) and the requester's side of the support desk (§16.2). */
export const supportApi = {
  createTicket: (input: TicketInput) => api<TicketReceipt>("/api/v1/support/tickets", { method: "POST", body: input }),
  /** Signed in: the account's requests, latest activity first. */
  myTickets: (cursor?: string, limit = 20, signal?: AbortSignal) =>
    api<Page<CustomerTicketRow>>("/api/v1/me/support/tickets", { query: { cursor, limit }, signal }),
  myTicket: (reference: string, signal?: AbortSignal) =>
    api<CustomerTicket>(`/api/v1/me/support/tickets/${encodeURIComponent(reference)}`, { signal }),
  replyMine: (reference: string, body: string) =>
    api<CustomerTicket>(`/api/v1/me/support/tickets/${encodeURIComponent(reference)}/messages`, { method: "POST", body: { body } }),
  /** Anonymous: through the private e-mailed link; the token travels in a header, never in the URL. */
  linkTicket: (reference: string, token: string, signal?: AbortSignal) =>
    api<CustomerTicket>(`/api/v1/support/tickets/${encodeURIComponent(reference)}`, { headers: { "X-Support-Token": token }, signal }),
  replyLink: (reference: string, token: string, body: string) =>
    api<CustomerTicket>(`/api/v1/support/tickets/${encodeURIComponent(reference)}/messages`, {
      method: "POST",
      body: { body },
      headers: { "X-Support-Token": token },
    }),
};

/** Public runtime settings: promo banners, sign-up offer, free mode (§15.1). */
export const siteApi = {
  config: (signal?: AbortSignal) => api<SiteConfig>("/api/v1/site/config", { signal, retries: 1 }),
};

export const astrologyApi = {
  places: (q: string, signal?: AbortSignal) => api<{ items: Place[] }>("/api/v1/astrology/places", { query: { q }, signal }).then((r) => r.items),
  chart: (input: { birth_date: string; birth_time?: string; place_id?: string }) =>
    api<Chart>("/api/v1/astrology/chart", { method: "POST", body: input, idempotent: true }),
  horoscope: (sign: string, date?: string) => api<Horoscope>(`/api/v1/astrology/horoscope/${encodeURIComponent(sign.toLowerCase())}`, { query: { date } }),
  events: (signal?: AbortSignal) => api<CosmicEvents>("/api/v1/astrology/events", { signal }),
};

// --- Advisor settings (§8.2) -----------------------------------------------------
/** Maximum advisor photo upload accepted by the API. */
export const ADVISOR_PHOTO_MAX_BYTES = 5 * 1024 * 1024;

/** The signed-in advisor's settings, photo review and custom offering. */
export const advisorApi = {
  settings: (signal?: AbortSignal) => api<AdvisorSettings>("/api/v1/psychic/settings", { signal }),
  /** Partial update; a stale `expected_version` answers 409 `version_conflict`. */
  updateSettings: (patch: AdvisorSettingsPatch) => api<AdvisorSettings>("/api/v1/psychic/settings", { method: "PATCH", body: patch }),
  photo: (signal?: AbortSignal) => api<PhotoState>("/api/v1/psychic/photo", { signal }),
  /** Submit a photo for review (multipart field "photo"); it stays private until approved. */
  uploadPhoto: (file: Blob, filename = "photo.jpg") => {
    const form = new FormData();
    form.append("photo", file, file instanceof File ? file.name : filename);
    return api<PhotoState>("/api/v1/psychic/photo", { method: "POST", body: form, timeoutMs: 60_000 });
  },
  /** Withdraw the pending submission. */
  withdrawPhoto: () => api<void>("/api/v1/psychic/photo", { method: "DELETE", idempotent: true }),
  /** The advisor's own pending or rejected photo (private; shown from an object URL). */
  photoImage: (signal?: AbortSignal) => api<Blob>("/api/v1/psychic/photo/image", { responseType: "blob", signal, timeoutMs: 30_000 }),
  customService: (signal?: AbortSignal) => api<CustomServiceState>("/api/v1/psychic/service", { signal }),
  /** Create (201) or replace (200) the custom offering. */
  saveCustomService: (input: CustomServiceInput) =>
    api<CustomServiceState>("/api/v1/psychic/service", { method: "PUT", body: input, idempotent: true }),
  deleteCustomService: () => api<void>("/api/v1/psychic/service", { method: "DELETE", idempotent: true }),
};
