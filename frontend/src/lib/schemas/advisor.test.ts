import { describe, expect, it } from "vitest"

import type { AdvisorSettings } from "@/lib/api/types"
import {
  aboutParagraphs,
  aboutText,
  advisorProfileSchema,
  advisorProfileValues,
  advisorSettingsPatch,
  customServiceSchema,
  customServiceValues,
  DEFAULT_CUSTOM_LIMITS,
  toCustomServiceInput,
} from "@/lib/schemas/advisor"

const text = <T,>(value: T, values?: Record<string, string | number>): T =>
  typeof value === "string" && values
    ? (value.replace(/\{(\w+)\}/g, (_, k: string) => String(values[k])) as T)
    : value

const saved: AdvisorSettings = {
  version: 4,
  phone: "+442079460958",
  phone_masked: "+••••••••••58",
  headline: "",
  about: ["First paragraph.", "Second one."],
  philosophy: "",
  modalities: ["Tarot", "Astrology"],
}

async function errors(
  schema: ReturnType<typeof advisorProfileSchema> | ReturnType<typeof customServiceSchema>,
  value: unknown,
  context?: Record<string, unknown>
) {
  try {
    await schema.validate(value, { abortEarly: false, context })
    return {}
  } catch (err) {
    const out: Record<string, string> = {}
    for (const e of (err as { inner: { path: string; message: string }[] }).inner) out[e.path] ??= e.message
    return out
  }
}

describe("about paragraphs", () => {
  it("splits on blank lines and drops empty paragraphs", () => {
    expect(aboutParagraphs("One\nstill one.\n\n  Two  \r\n\r\n\n\nThree")).toEqual(["One\nstill one.", "Two", "Three"])
    expect(aboutParagraphs("   \n\n ")).toEqual([])
    expect(aboutText(["A", "B"])).toBe("A\n\nB")
    expect(aboutParagraphs(aboutText(saved.about))).toEqual(saved.about)
  })
})

describe("advisorProfileSchema", () => {
  const schema = advisorProfileSchema(text)

  it("accepts the saved values and normalizes the phone", async () => {
    const value = await schema.validate({ phone: " +44 20 7946-0958 ", about: "Hi", modalities: ["Tarot"] })
    expect(value.phone).toBe("+442079460958")
    expect(await errors(schema, { phone: "", about: "", modalities: [] })).toEqual({})
  })

  it("rejects a phone that is not E.164", async () => {
    for (const phone of ["020 7946 0958", "+12", "+0123456789", "phone"]) {
      expect((await errors(schema, { phone, about: "", modalities: [] })).phone).toMatch(/international format/)
    }
  })

  it("bounds paragraphs, paragraph length and modalities", async () => {
    const seven = Array.from({ length: 7 }, (_, i) => `P${i}`).join("\n\n")
    expect((await errors(schema, { phone: "", about: seven, modalities: [] })).about).toBe("Use at most 6 paragraphs.")
    expect((await errors(schema, { phone: "", about: "x".repeat(2001), modalities: [] })).about).toBe("Keep each paragraph under 2,000 characters.")
    expect(await errors(schema, { phone: "", about: "é".repeat(2000), modalities: [] })).toEqual({})
    const nine = Array.from({ length: 9 }, (_, i) => `M${i}`)
    expect((await errors(schema, { phone: "", about: "", modalities: nine })).modalities).toBe("Add up to 8 modalities.")
    expect((await errors(schema, { phone: "", about: "", modalities: ["x".repeat(41)] })).modalities).toMatch(/between 1 and 40/)
  })
})

describe("advisorSettingsPatch", () => {
  it("sends only what changed, with the loaded version", () => {
    const values = advisorProfileValues(saved)
    expect(advisorSettingsPatch(values, saved)).toBeNull()
    expect(advisorSettingsPatch({ ...values, phone: "+14155552671" }, saved)).toEqual({ phone: "+14155552671", expected_version: 4 })
    expect(advisorSettingsPatch({ ...values, phone: "" }, saved)).toEqual({ phone: "", expected_version: 4 })
    expect(advisorSettingsPatch({ ...values, about: "First paragraph.\n\n\nSecond one.\n" }, saved)).toBeNull()
    expect(advisorSettingsPatch({ ...values, about: "Only one." }, saved)).toEqual({ about: ["Only one."], expected_version: 4 })
    expect(advisorSettingsPatch({ ...values, modalities: ["Tarot", "Astrology", "Aura  reading"] }, saved)).toEqual({
      modalities: ["Tarot", "Astrology", "Aura reading"],
      expected_version: 4,
    })
  })
})

describe("customServiceSchema", () => {
  const schema = customServiceSchema(text)
  const valid = { name: "Soulmate sketch", price: "2000", days: "8", description: "" }

  it("requires a name, a whole price and whole days within the bounds", async () => {
    expect(await errors(schema, valid)).toEqual({})
    const e = await errors(schema, { name: " ", price: "", days: "", description: "" })
    expect(Object.keys(e).sort()).toEqual(["days", "name", "price"])
    expect((await errors(schema, { ...valid, price: "12.5" })).price).toBe("Enter a whole number of credits.")
    expect((await errors(schema, { ...valid, price: "9" })).price).toBe("Use a price between 10 and 50000 credits.")
    expect((await errors(schema, { ...valid, price: "50001" })).price).toMatch(/between 10 and 50000/)
    expect((await errors(schema, { ...valid, days: "0" })).days).toBe("Use between 1 and 60 days.")
    expect((await errors(schema, { ...valid, days: "61" })).days).toBe("Use between 1 and 60 days.")
    expect((await errors(schema, { ...valid, days: "2.5" })).days).toBe("Enter a whole number of days.")
    expect((await errors(schema, { ...valid, name: "n".repeat(81) })).name).toBe("Use at most 80 characters.")
    expect((await errors(schema, { ...valid, description: "d".repeat(1001) })).description).toBe("Use at most 1000 characters.")
  })

  it("uses the server's bounds and number format from the context", async () => {
    const context = { limits: { ...DEFAULT_CUSTOM_LIMITS, min_price_credits: 100, max_price_credits: 900 }, format: (n: number) => `#${n}` }
    expect((await errors(schema, { ...valid, price: "99" }, context)).price).toBe("Use a price between #100 and #900 credits.")
    expect(await errors(schema, { ...valid, price: "900" }, context)).toEqual({})
  })

  it("maps between the API and the form", () => {
    expect(customServiceValues(null)).toEqual({ name: "", price: "", days: "", description: "" })
    const service = { id: "s", name: "Sketch", description: "Pencil", price_credits: 2000, delivery_days: 8, active: true, created_at: "", updated_at: "" }
    expect(customServiceValues(service)).toEqual({ name: "Sketch", price: "2000", days: "8", description: "Pencil" })
    expect(toCustomServiceInput({ name: "  Soulmate   sketch ", price: " 2000 ", days: "8", description: "  " })).toEqual({
      name: "Soulmate sketch",
      price_credits: 2000,
      delivery_days: 8,
    })
    expect(toCustomServiceInput({ ...valid, description: " Pencil portrait " }).description).toBe("Pencil portrait")
  })
})
