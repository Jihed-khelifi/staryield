"use client";
import { useEffect } from "react";

import { captureAttribution } from "@/lib/attribution";

/** Records campaign landings (UTM / short-link click) once per page load. */
export function AttributionCapture() {
  useEffect(() => {
    captureAttribution();
  }, []);
  return null;
}
