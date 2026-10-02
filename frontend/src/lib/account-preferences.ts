"use client";
import { useMemo, useSyncExternalStore } from "react";

const key = "staryield.preferences";
const empty = JSON.stringify({
  name: "UserName",
  email: "",
  nickname: "",
  birth: "",
  location: "",
  mood: "Empowered",
  horoscope: true,
  events: false,
  chart: false,
  theme: "Warm Beige",
  avatar: "",
});
function subscribe(callback: () => void) {
  window.addEventListener("storage", callback);
  window.addEventListener("staryield-preferences", callback);
  return () => {
    window.removeEventListener("storage", callback);
    window.removeEventListener("staryield-preferences", callback);
  };
}
function snapshot() {
  try {
    return localStorage.getItem(key) ?? empty;
  } catch {
    return empty;
  }
}
export type AccountPreferences = {
  name: string;
  email: string;
  nickname: string;
  birth: string;
  location: string;
  mood: string;
  horoscope: boolean;
  events: boolean;
  chart: boolean;
  theme: string;
  avatar: string;
};
export function useAccountPreferences() {
  const raw = useSyncExternalStore(subscribe, snapshot, () => empty);
  const preferences = useMemo<AccountPreferences>(() => {
    try {
      return { ...JSON.parse(empty), ...JSON.parse(raw) };
    } catch {
      return JSON.parse(empty);
    }
  }, [raw]);
  function save(next: AccountPreferences) {
    localStorage.setItem(key, JSON.stringify(next));
    window.dispatchEvent(new Event("staryield-preferences"));
  }
  return { preferences, save };
}
