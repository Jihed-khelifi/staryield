import { describe, expect, it } from "vitest";

import type { Settings } from "@/lib/api/account";
import { anyDirty, buildSettingsPatch, toFormValues } from "@/components/account/settings/settings-model";

const settings: Settings = {
  version: 7,
  account: { email: "a@b.co", email_verified: true, display_name: "Jane", role: "client", created_at: "2026-01-01T00:00:00Z" },
  profile: {
    nickname: "", birth_date: "1990-03-14", birth_time: "", birth_place_id: "gn:1", birth_place: "Paris, France", birth_latitude: null,
    birth_longitude: null, birth_timezone: "", country_code: "FR", location: "", mood: "", house_system: "placidus", avatar_url: "",
  },
  appearance: { theme: "Warm Beige", locale: "en", time_zone: "" },
  notifications: { email: true, push: false, marketing: false, horoscope: false, events: false, chart: false },
  privacy: { share_birth_details_with_advisors: false, consents: {}, policy_version: "2026-11" },
};

describe("settings model", () => {
  it("maps a document to form values (browser zone, unknown time)", () => {
    const v = toFormValues(settings, "Europe/Paris");
    expect(v.appearance.time_zone).toBe("Europe/Paris");
    expect(v.profile.time_unknown).toBe(true);
    expect(v.profile.place).toEqual({ id: "gn:1", label: "Paris, France" });
  });

  it("sends nothing when nothing is dirty", () => {
    expect(buildSettingsPatch(toFormValues(settings, "UTC"), {}, 7)).toBeNull();
  });

  it("builds one patch of only the changed fields with expected_version", () => {
    const v = toFormValues(settings, "UTC");
    v.account.display_name = " Jane D ";
    v.notifications.push = true;
    v.profile.time_unknown = false;
    v.profile.birth_time = "08:45";
    const patch = buildSettingsPatch(
      v,
      { account: { display_name: true }, notifications: { push: true }, profile: { birth_time: true, time_unknown: true } },
      7
    );
    expect(patch).toEqual({
      expected_version: 7,
      account: { display_name: "Jane D" },
      profile: { birth_time: "08:45" },
      notifications: { push: true },
    });
  });

  it("clears the birth time when marked unknown and the place when removed", () => {
    const v = toFormValues(settings, "UTC");
    v.profile.birth_time = "";
    v.profile.place = null;
    const patch = buildSettingsPatch(v, { profile: { time_unknown: true, place: true } }, 3);
    expect(patch?.profile).toEqual({ birth_time: "", birth_place_id: "" });
  });

  it("detects nested dirtiness", () => {
    expect(anyDirty({ a: { b: [false, true] } })).toBe(true);
    expect(anyDirty({ a: false })).toBe(false);
  });
});
