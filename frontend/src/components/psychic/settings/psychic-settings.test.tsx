// @vitest-environment jsdom
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest"
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react"

import { ApiError } from "@/lib/api/errors"
import type { AdvisorSettings, CustomServiceState, PhotoState } from "@/lib/api/types"
import { ToastProvider } from "@/components/feedback/toast-provider"
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render"

const mocks = vi.hoisted(() => ({
  ADVISOR_PHOTO_MAX_BYTES: 5 * 1024 * 1024,
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
}))
vi.mock("@/lib/api/endpoints", () => mocks)
vi.mock("@/lib/auth/session-provider", () => ({
  useRealtimeEvent: () => undefined,
  useSession: () => ({ realtime: null, user: { display_name: "Ramone", psychic: { name: "Ramone" } } }),
}))

import { PsychicSettings } from "@/components/psychic/settings/psychic-settings"

const api = mocks.advisorApi

const settings = (over: Partial<AdvisorSettings> = {}): AdvisorSettings => ({
  version: 3,
  phone: "+442079460958",
  phone_masked: "+••••••••••58",
  headline: "",
  about: ["Reads palms and auras."],
  philosophy: "",
  modalities: ["Tarot", "Astrology"],
  ...over,
})
const limits = { max_bytes: 5 << 20, max_pixels: 40_000_000, min_side: 256, daily_uploads: 10 }
const photo = (over: Partial<PhotoState> = {}): PhotoState => ({ image_url: "", submission: null, limits, ...over })
const customs = (over: Partial<CustomServiceState> = {}): CustomServiceState => ({
  service: null,
  limits: { name_max: 80, description_max: 1000, min_price_credits: 10, max_price_credits: 50_000, min_delivery_days: 1, max_delivery_days: 60 },
  ...over,
})

beforeAll(() => {
  stubResizeObserver()
  URL.createObjectURL = vi.fn(() => "blob:preview")
  URL.revokeObjectURL = vi.fn()
})
afterEach(cleanup)
beforeEach(() => {
  for (const fn of Object.values(api)) fn.mockReset()
  api.settings.mockResolvedValue(settings())
  api.photo.mockResolvedValue(photo())
  api.customService.mockResolvedValue(customs())
  api.photoImage.mockResolvedValue(new Blob(["x"], { type: "image/jpeg" }))
})

function renderPage(ui = <PsychicSettings />) {
  return renderWithI18n(<ToastProvider>{ui}</ToastProvider>)
}

async function loaded() {
  await screen.findByTestId("advisor-profile-form")
  await screen.findByTestId("customs-form")
  await screen.findByTestId("profile-photo")
}

describe("PsychicSettings", () => {
  it("lays out the advisor cards and a placeholder for every shared section", async () => {
    renderPage()
    expect(screen.getByRole("heading", { level: 1, name: "Profile Settings" })).toBeTruthy()
    expect(screen.getByTestId("advisor-profile-loading")).toBeTruthy()
    for (const slot of ["account", "security", "privacy", "notifications", "appearance", "session"]) {
      expect(screen.getByTestId(`settings-slot-${slot}`)).toBeTruthy()
    }
    await loaded()
    expect((screen.getByLabelText("Phone number") as HTMLInputElement).value).toBe("+442079460958")
    expect((screen.getByLabelText("About section") as HTMLTextAreaElement).value).toBe("Reads palms and auras.")
    expect(screen.getByText("Tarot")).toBeTruthy()
    expect(screen.getByTestId("settings-save")).toHaveProperty("disabled", true)
    expect(screen.getByRole("link", { name: "Back to Dashboard" }).getAttribute("href")).toBe("/en/profile")
    expect(await axeViolations()).toEqual([])
  })

  it("renders shared sections passed in by the account page", async () => {
    renderPage(<PsychicSettings shared={{ account: <section data-testid="shared-account">Name and e-mail</section> }} />)
    expect(screen.getByTestId("shared-account")).toBeTruthy()
    expect(screen.queryByTestId("settings-slot-account")).toBeNull()
    expect(screen.getByTestId("settings-slot-security")).toBeTruthy()
    await loaded()
  })

  it("validates, then saves only the changed fields with the loaded version", async () => {
    api.updateSettings.mockImplementation(async (patch) => settings({ version: 4, phone: patch.phone ?? "+442079460958", modalities: patch.modalities ?? ["Tarot", "Astrology"] }))
    renderPage()
    await loaded()
    const phone = screen.getByLabelText("Phone number")
    fireEvent.change(phone, { target: { value: "020 7946 0958" } })
    const save = screen.getByTestId("settings-save")
    expect(save).toHaveProperty("disabled", false)
    fireEvent.click(save)
    expect(await screen.findByText(/international format, e\.g\. \+14155552671\./, { selector: "[data-slot=form-message]" })).toBeTruthy()
    expect(api.updateSettings).not.toHaveBeenCalled()
    await waitFor(() => expect(document.activeElement).toBe(phone))

    fireEvent.change(phone, { target: { value: "+1 415 555 2671" } })
    const modality = screen.getByRole("textbox", { name: "Modalities" })
    fireEvent.change(modality, { target: { value: "Aura reading" } })
    fireEvent.keyDown(modality, { key: "Enter" })
    fireEvent.click(screen.getByTestId("settings-save"))
    await waitFor(() => expect(api.updateSettings).toHaveBeenCalledTimes(1))
    expect(api.updateSettings).toHaveBeenCalledWith({ phone: "+14155552671", modalities: ["Tarot", "Astrology", "Aura reading"], expected_version: 3 })
    expect(await screen.findByText("Your changes were saved.")).toBeTruthy()
    await waitFor(() => expect(screen.getByTestId("settings-save")).toHaveProperty("disabled", true))
  })

  it("reports a save made elsewhere and reloads on request", async () => {
    api.updateSettings.mockRejectedValue(
      new ApiError({ code: "version_conflict", message: "conflict", status: 409, details: { current_version: 4 } })
    )
    renderPage()
    await loaded()
    fireEvent.change(screen.getByLabelText("About section"), { target: { value: "A new story." } })
    fireEvent.click(screen.getByTestId("settings-save"))
    const alert = await screen.findByText(/changed in another tab or on another device/)
    expect(alert).toBeTruthy()
    api.settings.mockResolvedValue(settings({ version: 4, about: ["Saved elsewhere."] }))
    fireEvent.click(screen.getByRole("button", { name: "Reload settings" }))
    await waitFor(() => expect((screen.getByLabelText("About section") as HTMLTextAreaElement).value).toBe("Saved elsewhere."))
  })

  it("maps server field errors onto the fields", async () => {
    api.updateSettings.mockRejectedValue(
      new ApiError({ code: "validation_failed", message: "x", status: 422, details: { fields: { modalities: "Use at most 8 skills." } } })
    )
    renderPage()
    await loaded()
    fireEvent.change(screen.getByLabelText("About section"), { target: { value: "Changed." } })
    fireEvent.click(screen.getByTestId("settings-save"))
    expect(await screen.findByText("Use at most 8 skills.")).toBeTruthy()
  })

  it("creates the custom offering from the empty state and removes it", async () => {
    const created = {
      id: "svc-1",
      name: "Soulmate sketch",
      description: "",
      price_credits: 2000,
      delivery_days: 8,
      active: true,
      created_at: "2026-11-05T10:00:00Z",
      updated_at: "2026-11-05T10:00:00Z",
    }
    api.saveCustomService.mockResolvedValue(customs({ service: created }))
    renderPage()
    await loaded()
    const card = screen.getByTestId("customs-card")
    expect(within(card).getByTestId("customs-empty")).toBeTruthy()
    fireEvent.change(within(card).getByLabelText("Custom name"), { target: { value: "Soulmate sketch" } })
    fireEvent.change(within(card).getByLabelText("Custom rate"), { target: { value: "5" } })
    fireEvent.change(within(card).getByLabelText("Average delivery time"), { target: { value: "8" } })
    fireEvent.click(screen.getByTestId("settings-save"))
    expect(await within(card).findByText("Use a price between 10 and 50,000 credits.")).toBeTruthy()
    expect(api.saveCustomService).not.toHaveBeenCalled()

    fireEvent.change(within(card).getByLabelText("Custom rate"), { target: { value: "2000" } })
    fireEvent.click(screen.getByTestId("settings-save"))
    await waitFor(() => expect(api.saveCustomService).toHaveBeenCalledWith({ name: "Soulmate sketch", price_credits: 2000, delivery_days: 8 }))
    expect(await screen.findByText("Your custom offering is now on your profile.")).toBeTruthy()

    api.deleteCustomService.mockResolvedValue(undefined)
    fireEvent.click(await within(card).findByRole("button", { name: "Remove the custom offering" }))
    const dialog = await screen.findByRole("alertdialog")
    expect(within(dialog).getByText("Remove “Soulmate sketch”?")).toBeTruthy()
    fireEvent.click(within(dialog).getByRole("button", { name: "Remove offering" }))
    await waitFor(() => expect(api.deleteCustomService).toHaveBeenCalled())
    expect(await within(card).findByTestId("customs-empty")).toBeTruthy()
  })

  it("shows a pending photo privately, with its status, and withdraws it", async () => {
    api.photo.mockResolvedValue(
      photo({
        image_url: "https://advisorsassets.staryield.net/advisors/old.jpg",
        submission: { id: "p1", status: "pending", width: 640, height: 640, bytes: 1000, has_image: true, submitted_at: "2026-11-05T10:00:00Z" },
      })
    )
    api.withdrawPhoto.mockResolvedValue(undefined)
    renderPage()
    await loaded()
    const section = screen.getByTestId("profile-photo")
    expect(within(section).getByText("Pending review")).toBeTruthy()
    const img = await within(section).findByRole("img", { name: "Your new photo, waiting for review" })
    expect(img.getAttribute("src")).toBe("blob:preview")
    expect(api.photoImage).toHaveBeenCalled()
    api.photo.mockResolvedValue(photo({ image_url: "https://advisorsassets.staryield.net/advisors/old.jpg" }))
    fireEvent.click(within(section).getByRole("button", { name: "Withdraw" }))
    await waitFor(() => expect(api.withdrawPhoto).toHaveBeenCalled())
    expect(await within(screen.getByTestId("profile-photo")).findByRole("img", { name: "Profile photo of Ramone" })).toBeTruthy()
  })

  it("shows why a photo was declined", async () => {
    api.photo.mockResolvedValue(
      photo({
        submission: {
          id: "p2",
          status: "rejected",
          reason: "Your face is not visible.",
          width: 640,
          height: 640,
          bytes: 1000,
          has_image: false,
          submitted_at: "2026-11-05T10:00:00Z",
          reviewed_at: "2026-11-05T11:00:00Z",
        },
      })
    )
    renderPage()
    await loaded()
    const section = screen.getByTestId("profile-photo")
    expect(within(section).getByText("Not approved")).toBeTruthy()
    expect(within(section).getByTestId("photo-reason").textContent).toContain("Your face is not visible.")
    expect(within(section).getByRole("button", { name: "Upload another photo" })).toBeTruthy()
    expect(api.photoImage).not.toHaveBeenCalled() // the image was already cleaned up
  })

  it("checks the picked file before cropping", async () => {
    renderPage()
    await loaded()
    const input = screen.getByTestId("photo-input") as HTMLInputElement
    fireEvent.change(input, { target: { files: [new File(["gif"], "a.gif", { type: "image/gif" })] } })
    expect(await screen.findByText("Use a PNG, JPEG or WebP image.")).toBeTruthy()
    const big = new File([new Uint8Array(1)], "big.jpg", { type: "image/jpeg" })
    Object.defineProperty(big, "size", { value: 6 * 1024 * 1024 })
    fireEvent.change(input, { target: { files: [big] } })
    expect(await screen.findByText("Choose an image smaller than 5 MB.")).toBeTruthy()
    fireEvent.change(input, { target: { files: [new File(["png"], "me.png", { type: "image/png" })] } })
    const dialog = await screen.findByRole("dialog", { name: "Crop your photo" })
    expect(within(dialog).getByText(/upper half of the image/)).toBeTruthy()
    fireEvent.click(within(dialog).getByRole("button", { name: "Cancel" }))
    await waitFor(() => expect(screen.queryByRole("dialog", { name: "Crop your photo" })).toBeNull())
    expect(api.uploadPhoto).not.toHaveBeenCalled()
  })

  it("shows load errors with a retry in each card", async () => {
    api.settings.mockRejectedValue(new ApiError({ code: "internal_error", message: "boom", status: 500 }))
    api.customService.mockRejectedValue(new ApiError({ code: "network_error", message: "offline" }))
    renderPage()
    expect(await screen.findByText("Your profile settings could not be loaded.")).toBeTruthy()
    expect(await screen.findByText("Your custom offering could not be loaded.")).toBeTruthy()
    api.customService.mockResolvedValue(customs())
    const retry = within(screen.getByTestId("customs-card")).getByRole("button", { name: "Try again" })
    fireEvent.click(retry)
    expect(await screen.findByTestId("customs-form")).toBeTruthy()
  })
})
