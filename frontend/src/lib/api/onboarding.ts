/**
 * The client's "Get started" checklist (growth: lifecycle and onboarding).
 * Wire types mirror `backend/internal/service/onboarding.go`: the API computes
 * every step from the account itself (one query), so the browser never decides
 * what is done. A step whose feature is not available in the deployment (e-mail
 * verification, the referral program) is not listed.
 */
import { api } from "@/lib/api/client";

export type OnboardingStepKey = "verify_email" | "birth_details" | "first_reading" | "invite_friend";

export type OnboardingStep = { key: OnboardingStepKey; done: boolean };

/** GET /me/onboarding (and the body of `onboarding.updated`). */
export type Onboarding = {
  /** In display order. */
  steps: OnboardingStep[];
  completed: number;
  total: number;
  /** The client hid the checklist (it comes back with "Undo" or never). */
  dismissed: boolean;
};

/** Durable realtime event: the checklist was hidden or shown in another tab or device. */
export const ONBOARDING_UPDATED = "onboarding.updated";

const PATH = "/api/v1/me/onboarding";

export const onboardingApi = {
  get: (signal?: AbortSignal) => api<Onboarding>(PATH, { signal }),
  /** Hide the checklist. Idempotent on the server, so a retry is safe. */
  dismiss: () => api<Onboarding>(`${PATH}/dismiss`, { method: "POST", idempotent: true, retries: 1 }),
  /** Show it again (the "Undo" of the dismissal). */
  restore: () => api<Onboarding>(`${PATH}/dismiss`, { method: "DELETE", idempotent: true, retries: 1 }),
};
