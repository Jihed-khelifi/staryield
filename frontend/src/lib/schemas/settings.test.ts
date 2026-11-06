import { describe, expect, it } from "vitest";

import { fieldErrors, identityText } from "@/test/schema";
import {
  changePasswordSchema,
  closeAccountSchema,
  emailChangeSchema,
  isCalendarDate,
  isTimeZone,
  isoDay,
  latestBirthDate,
  passwordConfirmSchema,
  settingsSchema,
} from "@/lib/schemas/settings";

const valid = {
  account: { display_name: "Cassandra Moon" },
  profile: { birth_date: "1990-03-14", birth_time: "08:45", time_unknown: false, place: { id: "gn:2988507", label: "Paris, France" } as { id: string; label: string } | null },
  notifications: { email: true, push: false, marketing: false, horoscope: true, events: false, chart: false },
  privacy: { share_birth_details_with_advisors: false },
  appearance: { theme: "Warm Beige", locale: "en", time_zone: "Europe/Paris" },
};

const withProfile = (profile: Partial<typeof valid.profile>) => ({ ...valid, profile: { ...valid.profile, ...profile } });
const schema = settingsSchema(identityText);

describe("settingsSchema", () => {
  it("accepts a complete profile", () => {
    expect(fieldErrors(schema, valid)).toEqual({});
  });

  it("requires a name of at most 80 characters (counted in runes)", () => {
    expect(fieldErrors(schema, { ...valid, account: { display_name: "  " } })["account.display_name"]).toBe("This field is required.");
    expect(fieldErrors(schema, { ...valid, account: { display_name: "é".repeat(81) } })["account.display_name"]).toBe("Use at most 80 characters.");
    expect(fieldErrors(schema, { ...valid, account: { display_name: "✨".repeat(80) } })).toEqual({});
  });

  it("rejects impossible, future and under-age birth dates", () => {
    expect(fieldErrors(schema, withProfile({ birth_date: "2023-02-30" }))["profile.birth_date"]).toBe("Enter a valid date.");
    expect(fieldErrors(schema, withProfile({ birth_date: "1899-12-31" }))["profile.birth_date"]).toBe(
      "Enter a valid birth date that is not in the future."
    );
    const tomorrow = new Date();
    tomorrow.setDate(tomorrow.getDate() + 1);
    expect(fieldErrors(schema, withProfile({ birth_date: isoDay(tomorrow) }))["profile.birth_date"]).toBe(
      "Enter a valid birth date that is not in the future."
    );
    const minor = new Date();
    minor.setFullYear(minor.getFullYear() - 17);
    expect(fieldErrors(schema, withProfile({ birth_date: isoDay(minor) }))["profile.birth_date"]).toBe("You must be at least 18 years old.");
    expect(fieldErrors(schema, withProfile({ birth_date: latestBirthDate() }))).toEqual({});
  });

  it("allows clearing the birth data", () => {
    expect(fieldErrors(schema, withProfile({ birth_date: "", birth_time: "", place: null }))).toEqual({});
  });

  it("needs a 24-hour time, a birthplace and a date when a time is given", () => {
    expect(fieldErrors(schema, withProfile({ birth_time: "8:45" }))["profile.birth_time"]).toBe("Use the 24-hour format HH:MM.");
    expect(fieldErrors(schema, withProfile({ place: null }))["profile.place"]).toBe(
      "Choose where you were born so we can place your rising sign."
    );
    expect(fieldErrors(schema, withProfile({ birth_date: "" }))["profile.birth_date"]).toBe("Add your date of birth to use a birth time.");
  });

  it("does not ask for a place when the time is unknown", () => {
    expect(fieldErrors(schema, withProfile({ time_unknown: true, place: null }))).toEqual({});
  });

  it("only accepts the supported themes, languages and real time zones", () => {
    expect(fieldErrors(schema, { ...valid, appearance: { ...valid.appearance, theme: "Midnight" } })["appearance.theme"]).toBe(
      "Choose a theme from the list."
    );
    expect(fieldErrors(schema, { ...valid, appearance: { ...valid.appearance, locale: "de" } })["appearance.locale"]).toBe(
      "Choose English, French or Spanish."
    );
    expect(fieldErrors(schema, { ...valid, appearance: { ...valid.appearance, time_zone: "Mars/Olympus" } })["appearance.time_zone"]).toBe(
      "Choose a time zone from the list."
    );
    expect(fieldErrors(schema, { ...valid, appearance: { ...valid.appearance, time_zone: "" } })).toEqual({});
  });
});

describe("date and zone helpers", () => {
  it("validates calendar dates and IANA zones", () => {
    expect(isCalendarDate("2024-02-29")).toBe(true);
    expect(isCalendarDate("2023-02-29")).toBe(false);
    expect(isCalendarDate("29/02/2024")).toBe(false);
    expect(isTimeZone("America/Los_Angeles")).toBe(true);
    expect(isTimeZone("Not/AZone")).toBe(false);
    expect(isTimeZone("")).toBe(true);
  });
});

describe("changePasswordSchema", () => {
  const s = changePasswordSchema(identityText);
  it("requires the current password, a strong new one and a matching confirmation", () => {
    expect(fieldErrors(s, { current_password: "", new_password: "weak", confirm_password: "nope" })).toEqual({
      current_password: "Enter your current password.",
      new_password: "Use 8+ characters, an uppercase letter, a number, and a special character.",
      confirm_password: "The passwords do not match.",
    });
  });
  it("refuses reusing the current password", () => {
    expect(fieldErrors(s, { current_password: "Old!Pass123", new_password: "Old!Pass123", confirm_password: "Old!Pass123" }).new_password).toBe(
      "Choose a password different from the current one."
    );
    expect(fieldErrors(s, { current_password: "Old!Pass123", new_password: "New!Pass456", confirm_password: "New!Pass456" })).toEqual({});
  });
});

describe("emailChangeSchema", () => {
  const s = emailChangeSchema(identityText);
  it("needs a valid, different address and the password", () => {
    expect(fieldErrors(s, { email: "nope", password: "" })).toEqual({ email: "Enter a valid email address.", password: "Enter your password." });
    expect(fieldErrors(s, { email: " Jane@Example.com ", password: "x" }, { currentEmail: "jane@example.com" }).email).toBe(
      "This is already your email address."
    );
    expect(fieldErrors(s, { email: "new@example.com", password: "x" }, { currentEmail: "jane@example.com" })).toEqual({});
  });
});

describe("re-authentication and closure", () => {
  it("asks for the password", () => {
    expect(fieldErrors(passwordConfirmSchema(identityText), { password: "" })).toEqual({ password: "Enter your password." });
  });
  it("needs the exact word DELETE and limits the reason", () => {
    const s = closeAccountSchema(identityText);
    expect(fieldErrors(s, { password: "x", confirmation: "delete", reason: "" }).confirmation).toBe("Type DELETE to confirm.");
    expect(fieldErrors(s, { password: "x", confirmation: " DELETE ", reason: "" })).toEqual({});
    expect(fieldErrors(s, { password: "x", confirmation: "DELETE", reason: "a".repeat(501) }).reason).toBe("Use at most 500 characters.");
  });
});
