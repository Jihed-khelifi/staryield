"use client";
import { useEffect } from "react";

import { captureAttribution } from "@/lib/attribution";
import { captureReferral } from "@/lib/referral";

/** Records campaign landings (UTM / short-link click) and invite links (`?ref=`) once per page load. */
export function AttributionCapture() {
  useEffect(() => {
    captureAttribution();
    captureReferral(); // growth: referrals
  }, []);
  return null;
}
