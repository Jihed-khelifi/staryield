/**
 * The invite program (growth: referrals). Wire types mirror
 * `backend/internal/service/referrals.go` (`MyReferrals`). The API answers
 * counts only: it never says who signed up with the link or who had a
 * reading, and the client never asks.
 */
import { api } from "@/lib/api/client";

/** The program as the inviter sees it. */
export type ReferralProgramView = {
  enabled: boolean;
  /** Free minutes earned per friend who completes a reading. */
  reward_minutes: number;
  /** How many friends can earn a reward (per inviter). */
  max_rewards: number;
  /** Rewards were launched: earned minutes are added right away. Before, they wait for the launch. */
  launched: boolean;
  launched_at?: string;
};

/** The inviter's counters. */
export type ReferralStats = {
  /** Friends who signed up with the link (and were not ruled out). */
  joined: number;
  /** Friends who verified their e-mail and completed a reading (the reward is earned). */
  qualified: number;
  /** Earned minutes waiting for the launch (or for the next grant run). */
  pending_minutes: number;
  /** Minutes already added to the wallet. */
  granted_minutes: number;
  /** Rewards left under the cap. */
  remaining_rewards: number;
};

/** GET /me/referrals. */
export type MyReferrals = {
  code: string;
  /** `{site}/{locale}?ref=CODE`, ready to share. */
  link: string;
  program: ReferralProgramView;
  stats: ReferralStats;
};

export const referralsApi = {
  /** Clients only; creates the code on the first call. Private, never cached. */
  mine: (signal?: AbortSignal) => api<MyReferrals>("/api/v1/me/referrals", { signal, retries: 1 }),
};
