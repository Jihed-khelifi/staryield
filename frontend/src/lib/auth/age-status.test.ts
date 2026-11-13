import { describe, expect, it } from "vitest";

import { ageLanding, ageStatusOf, birthStepHref, isBirthStepPath, restrictedHref } from "@/lib/auth/age-status";
import { safeNext } from "@/lib/navigation";

// revisions 5.0: 18+ age gate (docs §24.6).
describe("age status helpers", () => {
  it("reads the status of a user and treats a missing one as adult (an API without the gate)", () => {
    expect(ageStatusOf({ age_status: "restricted" })).toBe("restricted");
    expect(ageStatusOf({ age_status: "birth_date_required" })).toBe("birth_date_required");
    expect(ageStatusOf({ age_status: "adult" })).toBe("adult");
    expect(ageStatusOf(null)).toBe("adult");
    expect(ageStatusOf({} as { age_status: "adult" })).toBe("adult");
  });

  it("builds the birth step URL with the page the person was heading to", () => {
    expect(birthStepHref("en")).toBe("/en/signup/birth");
    expect(birthStepHref("fr", "/fr/chatroom?reader=theo")).toBe(`/fr/signup/birth?next=${encodeURIComponent("/fr/chatroom?reader=theo")}`);
  });

  it("round-trips through the birth step's safe-next rule", () => {
    const href = birthStepHref("es", "/es/credits?pack=2#top");
    expect(safeNext(href.slice(href.indexOf("?")), "es")).toBe("/es/credits?pack=2#top");
  });

  it("points a restricted account at the restricted screen", () => {
    expect(restrictedHref("es")).toBe("/es/age-restricted");
  });

  it("routes a fresh sign-in by age", () => {
    expect(ageLanding({ age_status: "adult" }, "en", "/en/profile")).toBe("/en/profile");
    expect(ageLanding({ age_status: "birth_date_required" }, "en", "/en/chatroom")).toBe(`/en/signup/birth?next=${encodeURIComponent("/en/chatroom")}`);
    expect(ageLanding({ age_status: "restricted" }, "fr", "/fr/chatroom")).toBe("/fr/age-restricted");
  });

  it("recognises the birth step so nothing redirects to itself", () => {
    expect(isBirthStepPath("/en/signup/birth", "en")).toBe(true);
    expect(isBirthStepPath("/fr/signup/birth", "en")).toBe(false);
    expect(isBirthStepPath("/en/signup/offer", "en")).toBe(false);
  });
});
