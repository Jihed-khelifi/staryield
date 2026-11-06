/**
 * Account self-service API (identity module): the settings document, signed-in
 * devices, e-mail change, consents, personal-data exports, deactivation,
 * closure and Web Push. Wire types mirror `backend/internal/service/
 * {account,devices,consents,exports,account_lifecycle,push}.go`.
 *
 * One-time tokens (e-mail links, reactivation) only ever travel in request
 * bodies, never in query strings, so they stay out of logs and referrers.
 */
import { api, setAuthHint, setCsrfToken } from "@/lib/api/client";
import type { Page, SessionResponse } from "@/lib/api/types";

// --- Settings (GET/PATCH /me/settings) ---------------------------------------

export type ThemeName = "Warm Beige" | "Light Cream";
export type HouseSystem = "placidus" | "whole_sign" | "equal";
export type ConsentCategory = "analytics" | "marketing_attribution" | "personalization";

export type SettingsAccount = {
  email: string;
  email_verified: boolean;
  /** Masked address of an e-mail change awaiting confirmation. */
  pending_email?: string;
  display_name: string;
  role: "client" | "psychic" | "admin";
  created_at: string;
};

export type SettingsProfile = {
  nickname: string;
  birth_date: string;
  birth_time: string;
  birth_place_id: string;
  birth_place: string;
  birth_latitude: number | null;
  birth_longitude: number | null;
  birth_timezone: string;
  country_code: string;
  location: string;
  mood: string;
  house_system: string;
  avatar_url: string;
};

export type SettingsAppearance = {
  theme: string;
  locale: string;
  /** IANA zone; "" follows the browser. */
  time_zone: string;
};

export type SettingsNotifications = {
  email: boolean;
  push: boolean;
  marketing: boolean;
  horoscope: boolean;
  events: boolean;
  chart: boolean;
};

export type SettingsPrivacy = {
  share_birth_details_with_advisors: boolean;
  consents: Partial<Record<ConsentCategory, boolean>>;
  policy_version: string;
};

/** The whole settings page; `version` is sent back as `expected_version`. */
export type Settings = {
  version: number;
  account: SettingsAccount;
  profile: SettingsProfile;
  appearance: SettingsAppearance;
  notifications: SettingsNotifications;
  privacy: SettingsPrivacy;
};

/** A partial update; sections and fields left out are unchanged. */
export type SettingsPatch = {
  expected_version?: number;
  account?: { display_name?: string };
  profile?: {
    nickname?: string;
    birth_date?: string;
    birth_time?: string;
    birth_place_id?: string;
    birth_place_label?: string;
    location?: string;
    mood?: string;
    house_system?: string;
  };
  appearance?: { theme?: string; locale?: string; time_zone?: string };
  notifications?: Partial<SettingsNotifications>;
  privacy?: { share_birth_details_with_advisors?: boolean; consents?: Partial<Record<ConsentCategory, boolean>> };
};

// --- Devices -------------------------------------------------------------------

export type Device = {
  id: string;
  label: string;
  device_type: "desktop" | "mobile" | "tablet" | "bot" | "unknown" | (string & {});
  browser: string;
  os: string;
  /** The device making the request. */
  current: boolean;
  signed_in_at: string;
  last_seen_at: string;
  expires_at: string;
};

// --- Consents ------------------------------------------------------------------

export type ConsentPolicy = {
  version: string;
  categories: { category: "essential" | ConsentCategory; required: boolean; default: boolean; legal_basis: string }[];
  honor_gpc: boolean;
};

export type ConsentChoice = {
  category: "essential" | ConsentCategory;
  granted: boolean;
  required: boolean;
  /** False while the default applies. */
  explicit: boolean;
  policy_version?: string;
  source?: string;
  updated_at?: string;
};

export type Consents = { policy_version: string; items: ConsentChoice[]; outdated: boolean };

// --- Exports -------------------------------------------------------------------

export type ExportStatus = "queued" | "running" | "ready" | "failed" | "expired";

export type DataExport = {
  id: string;
  status: ExportStatus;
  requested_at: string;
  completed_at?: string;
  expires_at?: string;
  bytes?: number;
  sha256?: string;
  download_count: number;
};

export type DownloadLink = { token: string; expires_at: string; url: string };

// --- Lifecycle -----------------------------------------------------------------

export type ClosureResult = { status: string; deletion_scheduled_at: string };
export type EmailLinkResult = { status: "confirmed" | "cancelled" | "restored" };
/** Why an advisor cannot close the account yet (`closure_blocked`). */
export type ClosureBlocker = "earnings_balance" | "pending_withdrawals" | "open_requests";

// --- Push ------------------------------------------------------------------------

export type PushConfig = { enabled: boolean; public_key?: string };

function signedIn(res: SessionResponse): SessionResponse {
  setCsrfToken(res.csrf_token);
  setAuthHint(true);
  return res;
}

export const settingsApi = {
  get: (signal?: AbortSignal) => api<Settings>("/api/v1/me/settings", { signal }),
  /** One transaction; 409 version_conflict when `expected_version` is stale. */
  update: (patch: SettingsPatch) => api<Settings>("/api/v1/me/settings", { method: "PATCH", body: patch }),
};

export const devicesApi = {
  list: (cursor?: string, limit = 5, signal?: AbortSignal) => api<Page<Device>>("/api/v1/me/sessions", { query: { cursor, limit }, signal }),
  /** Signs one device out (204). Revoking the current device signs this browser out. */
  revoke: (id: string) => api<void>(`/api/v1/me/sessions/${encodeURIComponent(id)}`, { method: "DELETE", idempotent: true }),
  revokeOthers: () => api<{ signed_out: number }>("/api/v1/me/sessions", { method: "DELETE", query: { scope: "others" }, idempotent: true }),
};

export const emailChangeApi = {
  /** Always 202 (no account enumeration); the new address gets a confirmation link. */
  request: (email: string, password: string) =>
    api<{ status: string }>("/api/v1/me/email-change", { method: "POST", body: { email, password } }),
  confirm: (token: string) =>
    api<EmailLinkResult>("/api/v1/auth/email-change/confirm", { method: "POST", body: { token }, skipRefresh: true }),
  cancel: (token: string) =>
    api<EmailLinkResult>("/api/v1/auth/email-change/cancel", { method: "POST", body: { token }, skipRefresh: true }),
};

export const consentsApi = {
  policy: (signal?: AbortSignal) => api<ConsentPolicy>("/api/v1/consent-policy", { signal }),
  get: (signal?: AbortSignal) => api<Consents>("/api/v1/me/consents", { signal }),
  update: (consents: Partial<Record<ConsentCategory, boolean>>) =>
    api<Consents>("/api/v1/me/consents", { method: "PUT", body: { consents, source: "settings" }, idempotent: true }),
};

export const exportsApi = {
  /** 202; 409 export_in_progress; 429 limit_reached (with Retry-After). */
  request: (password: string) => api<DataExport>("/api/v1/me/exports", { method: "POST", body: { password } }),
  list: (cursor?: string, limit = 5, signal?: AbortSignal) => api<Page<DataExport>>("/api/v1/me/exports", { query: { cursor, limit }, signal }),
  get: (id: string, signal?: AbortSignal) => api<DataExport>(`/api/v1/me/exports/${encodeURIComponent(id)}`, { signal }),
  /** Single-use, short-lived token after re-authentication. */
  downloadToken: (id: string, password: string) =>
    api<DownloadLink>(`/api/v1/me/exports/${encodeURIComponent(id)}/download-token`, { method: "POST", body: { password } }),
  /**
   * Streams the archive. Never retried: the token is consumed by the first
   * request, so a retry could only fail.
   */
  download: (id: string, token: string) =>
    api<Blob>(`/api/v1/me/exports/${encodeURIComponent(id)}/download`, {
      query: { token },
      responseType: "blob",
      retries: 0,
      timeoutMs: 10 * 60_000,
    }),
};

export const accountApi = {
  /** Pauses the account and signs this browser out (204). */
  deactivate: (password: string) =>
    api<void>("/api/v1/me/deactivate", { method: "POST", body: { password } }).finally(() => setAuthHint(false)),
  /** Schedules erasure after the grace period (202); 409 closure_blocked for advisors with open business. */
  close: (input: { password: string; confirmation: string; reason?: string }) =>
    api<ClosureResult>("/api/v1/me/close", { method: "POST", body: input }),
  /** Consumes the token offered at sign-in and signs the user in. */
  reactivate: (token: string) =>
    api<SessionResponse>("/api/v1/auth/reactivate", { method: "POST", body: { token }, skipRefresh: true }).then(signedIn),
  /** Keeps an account from the e-mailed link (does not sign in). */
  cancelClosure: (token: string) =>
    api<{ status: string }>("/api/v1/auth/closure/cancel", { method: "POST", body: { token }, skipRefresh: true }),
};

export const pushApi = {
  config: (signal?: AbortSignal) => api<PushConfig>("/api/v1/push/config", { signal }),
  subscribe: (subscription: PushSubscriptionJSON) =>
    api<{ id: string; created_at: string }>("/api/v1/me/push-subscriptions", {
      method: "POST",
      body: { endpoint: subscription.endpoint, keys: subscription.keys, expirationTime: subscription.expirationTime ?? null },
      idempotent: true,
    }),
  unsubscribe: (endpoint: string) =>
    api<void>("/api/v1/me/push-subscriptions", { method: "DELETE", body: { endpoint }, idempotent: true }),
};
