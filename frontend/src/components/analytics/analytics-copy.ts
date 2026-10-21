/** English source copy for the analytics dashboard (translated with text()). */
import type { PsychicAnalytics, Withdrawal, WithdrawalMethod } from "@/lib/api/types";
import type { HeatCell, RangeKey } from "@/lib/psychic-analytics";

export const rangeCopy: Record<RangeKey, string> = {
  "7d": "7 days",
  "30d": "30 days",
  "90d": "90 days",
  "12m": "12 months",
};

export const gradeCopy: Record<PsychicAnalytics["score"]["grade"], { label: string; guidance: string }> = {
  excellent: {
    label: "Excellent",
    guidance: "Outstanding work. Clients love your readings: keep your availability steady to stay at the top.",
  },
  good: {
    label: "Good",
    guidance: "Great results. Answering requests faster and welcoming back returning clients can lift you to excellent.",
  },
  fair: {
    label: "Fair",
    guidance: "You are on your way. Accept more requests, answer within a minute and invite clients to come back.",
  },
  needs_attention: {
    label: "Needs attention",
    guidance: "Your score needs attention. Be online at your peak hours, answer requests quickly and let chats end gracefully.",
  },
};

export const componentCopy: { key: keyof PsychicAnalytics["score"]["components"]; label: string; weight: number }[] = [
  { key: "rating", label: "Rating", weight: 30 },
  { key: "acceptance", label: "Acceptance", weight: 25 },
  { key: "retention", label: "Returning clients", weight: 15 },
  { key: "completion", label: "Completed chats", weight: 15 },
  { key: "responsiveness", label: "Responsiveness", weight: 15 },
];

export const levelCopy: Record<HeatCell["level"], string> = { peak: "Peak", medium: "Medium", low: "Low", none: "No requests" };

export const withdrawalStatusLabel: Record<Withdrawal["status"], string> = {
  pending: "Pending review",
  approved: "Approved",
  paid: "Paid",
  rejected: "Rejected",
  cancelled: "Cancelled",
};

export const methodLabel: Record<WithdrawalMethod, string> = {
  bank_transfer: "Bank transfer",
  paypal: "PayPal",
};
