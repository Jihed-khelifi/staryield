// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import type { Settings } from "@/lib/api/account";
import { ApiError } from "@/lib/api/errors";
import { ToastProvider } from "@/components/feedback/toast-provider";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({
  settingsApi: { get: vi.fn(), update: vi.fn() },
  devicesApi: { list: vi.fn(), revoke: vi.fn(), revokeOthers: vi.fn() },
  exportsApi: { request: vi.fn(), list: vi.fn(), get: vi.fn(), downloadToken: vi.fn(), download: vi.fn() },
  consentsApi: { get: vi.fn(), update: vi.fn() },
  emailChangeApi: { request: vi.fn() },
  accountApi: { deactivate: vi.fn(), close: vi.fn() },
  pushApi: { config: vi.fn() },
  push: { availability: vi.fn(), enable: vi.fn() },
  billingApi: { plans: vi.fn(), subscription: vi.fn(), paymentMethods: vi.fn(), transactions: vi.fn(), clientConfig: vi.fn() },
  signOut: vi.fn(),
}));
vi.mock("@/lib/api/account", () => ({ ...m }));
vi.mock("@/lib/api/billing", () => ({ billingApi: m.billingApi }));
vi.mock("@/lib/api/endpoints", () => ({ astrologyApi: { places: vi.fn().mockResolvedValue([]) }, authApi: { changePassword: vi.fn() } }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn(), push: vi.fn() }), usePathname: () => "/en/profile/settings" }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ user: { id: "u1", role: "client" }, status: "authenticated", reload: vi.fn(), signOut: m.signOut }),
}));

vi.mock("@/lib/push", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/push")>()),
  pushAvailability: m.push.availability,
  enablePush: m.push.enable,
}));

import { AccountSettings } from "@/components/account/settings/account-settings";

const base: Settings = {
  version: 3,
  account: { email: "jane@example.com", email_verified: true, display_name: "Jane Doe", role: "client", created_at: "2026-01-01T00:00:00Z" },
  profile: {
    nickname: "", birth_date: "1990-03-14", birth_time: "08:45", birth_place_id: "gn:1", birth_place: "Paris, France", birth_latitude: null,
    birth_longitude: null, birth_timezone: "", country_code: "FR", location: "", mood: "", house_system: "placidus", avatar_url: "",
  },
  appearance: { theme: "Warm Beige", locale: "en", time_zone: "Europe/Paris" },
  notifications: { email: true, push: false, marketing: false, horoscope: false, events: false, chart: false },
  privacy: { share_birth_details_with_advisors: false, consents: {}, policy_version: "2026-11" },
};

beforeAll(stubResizeObserver);
beforeEach(() => {
  window.matchMedia = ((q: string) => ({ matches: false, media: q, addEventListener() {}, removeEventListener() {} })) as unknown as typeof window.matchMedia;
  m.settingsApi.get.mockResolvedValue(base);
  m.devicesApi.list.mockResolvedValue({ items: [{ id: "d1", label: "Chrome on macOS", device_type: "desktop", browser: "Chrome", os: "macOS", current: true, signed_in_at: "2026-11-01T10:00:00Z", last_seen_at: new Date().toISOString(), expires_at: "2026-12-01T00:00:00Z" }] });
  m.pushApi.config.mockResolvedValue({ enabled: false });
  m.push.availability.mockReset().mockReturnValue("unsupported");
  m.push.enable.mockReset();
  m.billingApi.plans.mockResolvedValue([]);
  m.billingApi.subscription.mockResolvedValue(null);
  m.billingApi.paymentMethods.mockResolvedValue([]);
  m.billingApi.transactions.mockResolvedValue({ items: [] });
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

function view() {
  return renderWithI18n(
    <ToastProvider>
      <AccountSettings />
    </ToastProvider>,
  );
}

describe("AccountSettings", () => {
  it("renders every section, is accessible, and has nothing to save at first", async () => {
    view();
    expect(await screen.findByRole("heading", { level: 1, name: /Profile Settings/ })).toBeTruthy();
    for (const name of ["Account Information", "Password & Security", "Data & Privacy", "Notifications", "Billing & Subscriptions", "Appearance", "Logout", "Account Actions"])
      expect(screen.getByRole("heading", { level: 2, name })).toBeTruthy();
    expect(await screen.findByText("Chrome on macOS (Current)")).toBeTruthy();
    expect(await axeViolations()).toEqual([]);
    await userEvent.setup().click(screen.getByTestId("settings-save"));
    expect(m.settingsApi.update).not.toHaveBeenCalled();
  });

  it("sends one PATCH with only the changed fields and the loaded version", async () => {
    const u = userEvent.setup();
    m.settingsApi.update.mockResolvedValue({ ...base, version: 4, account: { ...base.account, display_name: "Jane D" }, notifications: { ...base.notifications, marketing: true } });
    view();
    const name = await screen.findByLabelText("Full Name");
    await u.clear(name);
    await u.type(name, "Jane D");
    await u.click(screen.getByTestId("notify-marketing"));
    expect(screen.getByText("Unsaved changes", { selector: "[data-slot=save-status]" })).toBeTruthy();
    await u.click(screen.getByTestId("settings-save"));
    await waitFor(() => expect(m.settingsApi.update).toHaveBeenCalledTimes(1));
    expect(m.settingsApi.update).toHaveBeenCalledWith({ expected_version: 3, account: { display_name: "Jane D" }, notifications: { marketing: true } });
    expect(await screen.findByText("All changes saved", { selector: "[data-slot=save-status]" })).toBeTruthy();
  });

  it("validates before sending (blank name)", async () => {
    const u = userEvent.setup();
    view();
    await u.clear(await screen.findByLabelText("Full Name"));
    await u.click(screen.getByTestId("settings-save"));
    expect(await screen.findByText("This field is required.")).toBeTruthy();
    expect(m.settingsApi.update).not.toHaveBeenCalled();
  });

  it("shows server field errors on the matching field", async () => {
    const u = userEvent.setup();
    m.settingsApi.update.mockRejectedValue(new ApiError({ code: "validation_failed", message: "x", status: 422, details: { fields: { "profile.birth_time": "Use the 24-hour format HH:MM." } } }));
    view();
    await u.click(await screen.findByTestId("notify-chart"));
    await u.click(screen.getByTestId("settings-save"));
    expect(await screen.findAllByText("Use the 24-hour format HH:MM.")).toBeTruthy();
  });

  it("handles a version conflict: reload, or save over", async () => {
    const u = userEvent.setup();
    m.settingsApi.update.mockRejectedValueOnce(new ApiError({ code: "version_conflict", message: "x", status: 409, details: { current_version: 5 } }));
    view();
    await u.click(await screen.findByTestId("notify-events"));
    await u.click(screen.getByTestId("settings-save"));
    const dialog = await screen.findByTestId("conflict-dialog");
    // Overwrite: resends the changed fields against the current version.
    m.settingsApi.update.mockResolvedValueOnce({ ...base, version: 6, notifications: { ...base.notifications, events: true } });
    await u.click(within(dialog).getByTestId("conflict-overwrite"));
    await waitFor(() => expect(m.settingsApi.update).toHaveBeenCalledTimes(2));
    expect(m.settingsApi.update).toHaveBeenLastCalledWith({ expected_version: 5, notifications: { events: true } });
    await waitFor(() => expect(screen.queryByTestId("conflict-dialog")).toBeNull());
  });

  it("reloads the latest settings from the conflict dialog", async () => {
    const u = userEvent.setup();
    m.settingsApi.update.mockRejectedValueOnce(new ApiError({ code: "version_conflict", message: "x", status: 409, details: { current_version: 5 } }));
    view();
    await u.click(await screen.findByTestId("notify-events"));
    await u.click(screen.getByTestId("settings-save"));
    m.settingsApi.get.mockResolvedValue({ ...base, version: 5, account: { ...base.account, display_name: "From elsewhere" } });
    await u.click(await screen.findByTestId("conflict-reload"));
    expect(await screen.findByDisplayValue("From elsewhere")).toBeTruthy();
  });

  it("shows an error state with retry when the settings fail to load", async () => {
    m.settingsApi.get.mockRejectedValueOnce(new ApiError({ code: "service_unavailable", message: "x", status: 503 }));
    view();
    fireEvent.click(await screen.findByRole("button", { name: "Try again" }));
    expect(await screen.findByLabelText("Full Name")).toBeTruthy();
  });

  it("explains unsupported push and keeps the switch off", async () => {
    view();
    expect(await screen.findByTestId("push-hint")).toBeTruthy();
    expect(screen.getByTestId("notify-push").hasAttribute("disabled")).toBe(true);
  });

  it("shows the iOS Home Screen hint up-front instead of a dead switch", async () => {
    m.push.availability.mockReturnValue("ios_install");
    view();
    expect(await screen.findByTestId("push-hint")).toHaveProperty("textContent", expect.stringContaining("Add to Home Screen"));
    expect(screen.getByTestId("notify-push").hasAttribute("disabled")).toBe(true);
  });

  it("explains Brave with a copyable address, a Copy button and Try again", async () => {
    const u = userEvent.setup();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText } });
    m.push.availability.mockReturnValue("available");
    m.pushApi.config.mockResolvedValue({ enabled: true, public_key: "KEY" });
    m.push.enable.mockResolvedValueOnce("brave_blocked").mockResolvedValueOnce("subscribed");
    view();
    const toggle = await screen.findByTestId("notify-push");
    await waitFor(() => expect(toggle.hasAttribute("disabled")).toBe(false));
    await u.click(toggle);
    const hint = await screen.findByTestId("push-hint");
    expect(hint.textContent).toContain("Use Google services for push messaging");
    expect(screen.getByTestId("push-brave-address").textContent).toBe("brave://settings/privacy");
    await u.click(screen.getByTestId("push-brave-copy"));
    expect(writeText).toHaveBeenCalledWith("brave://settings/privacy");
    expect(await screen.findByText("Copied")).toBeTruthy();
    await u.click(screen.getByTestId("push-retry"));
    await waitFor(() => expect(screen.getByTestId("notify-push").getAttribute("aria-checked")).toBe("true"));
    expect(m.push.enable).toHaveBeenCalledTimes(2);
  });

  it("shows the service-error copy with a retry", async () => {
    const u = userEvent.setup();
    m.push.availability.mockReturnValue("available");
    m.pushApi.config.mockResolvedValue({ enabled: true, public_key: "KEY" });
    m.push.enable.mockResolvedValue("service_error");
    view();
    const toggle = await screen.findByTestId("notify-push");
    await waitFor(() => expect(toggle.hasAttribute("disabled")).toBe(false));
    await u.click(toggle);
    expect((await screen.findByTestId("push-hint")).textContent).toContain("notification service didn't respond");
    expect(screen.getByTestId("push-retry")).toBeTruthy();
  });

  it("pages devices and revokes one after confirmation", async () => {
    const u = userEvent.setup();
    m.devicesApi.list.mockResolvedValue({
      items: [
        { id: "d1", label: "", device_type: "desktop", browser: "Chrome", os: "macOS", current: true, signed_in_at: "2026-11-01T10:00:00Z", last_seen_at: new Date().toISOString(), expires_at: "2026-12-01T00:00:00Z" },
        { id: "d2", label: "", device_type: "mobile", browser: "Safari", os: "iPhone", current: false, signed_in_at: "2026-11-01T10:00:00Z", last_seen_at: "2026-11-01T10:00:00Z", expires_at: "2026-12-01T00:00:00Z" },
      ],
      next_cursor: "c2",
    });
    m.devicesApi.revoke.mockResolvedValue(undefined);
    view();
    await u.click(await screen.findByRole("button", { name: "Sign out Safari on iPhone" }));
    await u.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Sign out" }));
    await waitFor(() => expect(m.devicesApi.revoke).toHaveBeenCalledWith("d2"));
    expect(await screen.findByRole("button", { name: /Next/ })).toBeTruthy();
  });
});
