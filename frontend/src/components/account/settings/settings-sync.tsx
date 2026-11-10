"use client";

import * as React from "react";

import type { Settings } from "@/lib/api/account";

/**
 * Lets a control that saves on its own (the push switch, docs §23.6) tell the
 * settings form which version the server is at now, so the next "Save Changes"
 * does not send a stale `expected_version` and trip a version conflict.
 */
export type SettingsSync = { adopt: (saved: Settings) => void };

export const SettingsSyncContext = React.createContext<SettingsSync | null>(null);

export function useSettingsSync(): SettingsSync | null {
  return React.useContext(SettingsSyncContext);
}
