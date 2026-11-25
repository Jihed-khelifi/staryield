"use client";

import { ageStatusOf } from "@/lib/auth/age-status";
import { useSession } from "@/lib/auth/session-provider";
import type { BookingInput } from "@/lib/booking/booking-state";
import { useFreeMode, useSignupFreeMinutes } from "@/lib/site-config/site-config-provider";

/** What `bookingState` needs to know about the viewer and the site (the same for every advisor on the page). */
export type BookingViewer = Pick<BookingInput, "session" | "role" | "freeMinutes" | "signupOfferMinutes" | "freeMode" | "age"> & {
  /** A signed-in client: the only role that can chat or wait. */
  isClient: boolean;
};

/** Reads the viewer's booking facts, so the book button and the lines next to it agree. */
export function useBookingViewer(): BookingViewer {
  const { status, user, wallet } = useSession();
  const freeMode = useFreeMode();
  const signupOfferMinutes = useSignupFreeMinutes();
  return {
    session: status,
    role: user?.role ?? null,
    freeMinutes: wallet?.free_minutes ?? 0,
    signupOfferMinutes,
    freeMode,
    age: ageStatusOf(user),
    isClient: status === "authenticated" && user?.role === "client",
  };
}
