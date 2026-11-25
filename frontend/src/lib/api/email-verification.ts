/**
 * E-mail verification by secure link (growth). Wire types mirror
 * `backend/internal/service/email_verification.go`.
 *
 * The one-time token only ever travels in a request body (the link carries it
 * in the URL fragment, which no server, log or referrer sees).
 */
import { api } from "@/lib/api/client";

/** POST /me/email/verification (202): a new link was sent. */
export type VerificationSent = {
  status: "sent";
  /** Masked address the link went to ("m***@example.com"). */
  email: string;
  /** ISO time when another link can be requested. */
  resend_available_at: string;
};

/** POST /auth/email/verify (200). */
export type VerificationResult = {
  status: "verified" | "already_verified";
  email_verified: true;
};

export const emailVerificationApi = {
  /** Public (the link may be opened on another device, signed out). 400 `invalid_token` for unknown, expired or used links. */
  verify: (token: string) => api<VerificationResult>("/api/v1/auth/email/verify", { method: "POST", body: { token }, skipRefresh: true }),
  /** 409 `already_verified`; 429 `rate_limited` with Retry-After (cooldown, then an hourly budget). */
  resend: () => api<VerificationSent>("/api/v1/me/email/verification", { method: "POST" }),
};
