// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { cleanup, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import type * as React from "react"

import type { Settings } from "@/lib/api/account"
import { ApiError } from "@/lib/api/errors"
import type { AdvisorSettings, CustomServiceState, PhotoState } from "@/lib/api/types"
import { ToastProvider } from "@/components/feedback/toast-provider"
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render"

// The advisor Settings page hosting the shared account sections
// (SharedAccountSettings) behind its single "Save Changes".
const m = vi.hoisted(() => ({
  settingsApi: { get: vi.fn(), update: vi.fn() },
  devicesApi: { list: vi.fn(), revoke: vi.fn(), revokeOthers: vi.fn() },
  exportsApi: { request: vi.fn(), list: vi.fn(), get: vi.fn(), downloadToken: vi.fn(), download: vi.fn() },
  consentsApi: { get: vi.fn(), update: vi.fn() },
  emailChangeApi: { request: vi.fn() },
  accountApi: { deactivate: vi.fn(), close: vi.fn() },
  pushApi: { config: vi.fn() },
  billingApi: { plans: vi.fn(), subscription: vi.fn(), paymentMethods: vi.fn(), transactions: vi.fn(), clientConfig: vi.fn() },
  advisorApi: {
    settings: vi.fn(),
    updateSettings: vi.fn(),
    photo: vi.fn(),
    uploadPhoto: vi.fn(),
    withdrawPhoto: vi.fn(),
    photoImage: vi.fn(),
    customService: vi.fn(),
    saveCustomService: vi.fn(),
    deleteCustomService: vi.fn(),
  },
  signOut: vi.fn(),
}))
vi.mock("@/lib/api/account", () => ({
  settingsApi: m.settingsApi,
  devicesApi: m.devicesApi,
  exportsApi: m.exportsApi,
  consentsApi: m.consentsApi,
  emailChangeApi: m.emailChangeApi,
  accountApi: m.accountApi,
  pushApi: m.pushApi,
}))
vi.mock("@/lib/api/billing", () => ({ billingApi: m.billingApi }))
vi.mock("@/lib/api/endpoints", () => ({
  ADVISOR_PHOTO_MAX_BYTES: 5 * 1024 * 1024,
  advisorApi: m.advisorApi,
  astrologyApi: { places: vi.fn().mockResolvedValue([]) },
  authApi: { changePassword: vi.fn() },
  signInMethodsApi: { get: vi.fn().mockResolvedValue({ password: true, google_available: false }), disconnectGoogle: vi.fn() },
}))
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn(), push: vi.fn() }), usePathname: () => "/en/profile/settings" }))
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}))
vi.mock("@/lib/auth/session-provider", () => ({
  useRealtimeEvent: () => undefined,
  useSession: () => ({
    realtime: null,
    status: "authenticated",
    reload: vi.fn(),
    signOut: m.signOut,
    user: { id: "p1", role: "psychic", display_name: "Ramone", psychic: { name: "Ramone" } },
  }),
}))

import { SharedAccountSettings } from "@/components/account/settings/account-settings"
import { PsychicSettings } from "@/components/psychic/settings/psychic-settings"

const base: Settings = {
  version: 7,
  account: { email: "ramone@example.com", email_verified: true, display_name: "Ramone", role: "psychic", created_at: "2026-01-01T00:00:00Z" },
  profile: {
    nickname: "", birth_date: "1985-06-02", birth_time: "", birth_place_id: "", birth_place: "", birth_latitude: null,
    birth_longitude: null, birth_timezone: "", country_code: "GB", location: "", mood: "", house_system: "placidus", avatar_url: "",
  },
  appearance: { theme: "Warm Beige", locale: "en", time_zone: "Europe/London" },
  notifications: { email: true, push: false, marketing: false, horoscope: false, events: false, chart: false },
  privacy: { share_birth_details_with_advisors: false, consents: {}, policy_version: "2026-11" },
}

const advisor: AdvisorSettings = {
  version: 3,
  phone: "+442079460958",
  phone_masked: "+••••••••••58",
  headline: "",
  about: ["Reads palms and auras."],
  philosophy: "",
  modalities: ["Tarot", "Astrology"],
}
const photo: PhotoState = { image_url: "", submission: null, limits: { max_bytes: 5 << 20, max_pixels: 40_000_000, min_side: 256, daily_uploads: 10 } }
const customs: CustomServiceState = {
  service: null,
  limits: { name_max: 80, description_max: 1000, min_price_credits: 10, max_price_credits: 50_000, min_delivery_days: 1, max_delivery_days: 60 },
}

beforeAll(() => {
  stubResizeObserver()
  URL.createObjectURL = vi.fn(() => "blob:preview")
  URL.revokeObjectURL = vi.fn()
})
beforeEach(() => {
  window.matchMedia = ((q: string) => ({ matches: false, media: q, addEventListener() {}, removeEventListener() {} })) as unknown as typeof window.matchMedia
  m.settingsApi.get.mockResolvedValue(base)
  m.devicesApi.list.mockResolvedValue({ items: [] })
  m.pushApi.config.mockResolvedValue({ enabled: false })
  m.advisorApi.settings.mockResolvedValue(advisor)
  m.advisorApi.photo.mockResolvedValue(photo)
  m.advisorApi.customService.mockResolvedValue(customs)
  m.advisorApi.photoImage.mockResolvedValue(new Blob(["x"], { type: "image/jpeg" }))
})
afterEach(() => {
  cleanup()
  vi.clearAllMocks()
})

function view() {
  return renderWithI18n(
    <ToastProvider>
      <PsychicSettings SharedProvider={SharedAccountSettings} />
    </ToastProvider>
  )
}

describe("advisor Settings with the shared account sections", () => {
  it("renders the real shared cards around the advisor cards, with no placeholders", async () => {
    view()
    expect(await screen.findByLabelText("Full Name")).toBeTruthy()
    await screen.findByTestId("advisor-profile-form")
    await screen.findByTestId("customs-form")
    for (const name of ["Account Information", "Password & Security", "Data & Privacy", "Notifications", "Appearance", "Logout", "Account Actions"])
      expect(screen.getByRole("heading", { level: 2, name })).toBeTruthy()
    for (const slot of ["account", "security", "privacy", "notifications", "appearance", "session"]) expect(screen.queryByTestId(`settings-slot-${slot}`)).toBeNull()
    // Clients-only parts stay hidden for advisors.
    expect(screen.queryByRole("heading", { level: 2, name: "Billing & Subscriptions" })).toBeNull()
    expect(await axeViolations()).toEqual([])
  })

  it("a change in a shared section enables the page's Save Changes and sends one PATCH /me/settings", async () => {
    const u = userEvent.setup()
    m.settingsApi.update.mockResolvedValue({ ...base, version: 8, account: { ...base.account, display_name: "Ramone W" } })
    view()
    const save = await screen.findByTestId("settings-save")
    await screen.findByTestId("advisor-profile-form")
    expect((save as HTMLButtonElement).disabled).toBe(true)
    const name = await screen.findByLabelText("Full Name")
    await u.clear(name)
    await u.type(name, "Ramone W")
    await waitFor(() => expect((save as HTMLButtonElement).disabled).toBe(false))
    await u.click(save)
    await waitFor(() => expect(m.settingsApi.update).toHaveBeenCalledTimes(1))
    expect(m.settingsApi.update).toHaveBeenCalledWith({ expected_version: 7, account: { display_name: "Ramone W" } })
    // The advisor cards had nothing to save.
    expect(m.advisorApi.updateSettings).not.toHaveBeenCalled()
    await waitFor(() => expect((save as HTMLButtonElement).disabled).toBe(true))
  })

  it("does not send an invalid shared section and reports the field", async () => {
    const u = userEvent.setup()
    view()
    const save = await screen.findByTestId("settings-save")
    await u.clear(await screen.findByLabelText("Full Name"))
    await waitFor(() => expect((save as HTMLButtonElement).disabled).toBe(false))
    await u.click(save)
    expect(await screen.findByText("This field is required.")).toBeTruthy()
    expect(m.settingsApi.update).not.toHaveBeenCalled()
  })

  it("shows an error with a retry in place of the account sections when /me/settings fails", async () => {
    m.settingsApi.get.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline", status: 0 })).mockResolvedValueOnce(base)
    const u = userEvent.setup()
    view()
    expect(await screen.findByRole("button", { name: /try again/i })).toBeTruthy()
    // The advisor's own cards still work.
    expect(await screen.findByTestId("advisor-profile-form")).toBeTruthy()
    expect(screen.queryByLabelText("Full Name")).toBeNull()
    await u.click(screen.getByRole("button", { name: /try again/i }))
    expect(await screen.findByLabelText("Full Name")).toBeTruthy()
  })
})
