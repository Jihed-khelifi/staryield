// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import natal from "@/test/natal-fixture.json";
import type { NatalResult } from "@/lib/api/astrology";
import { ApiError } from "@/lib/api/errors";
import { lifePathReduction, localIsoDate, personalCycles } from "@/lib/calculators";
import { renderWithI18n, stubResizeObserver } from "@/test/render";

const m = vi.hoisted(() => ({
  natal: vi.fn(),
  sunSign: vi.fn(),
  track: vi.fn(),
  render: vi.fn(),
  prompt: vi.fn(),
}));
vi.mock("@/lib/api/astrology", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/api/astrology")>()),
  astroApi: { natal: m.natal, sunSign: m.sunSign, place: vi.fn() },
}));
vi.mock("@/lib/analytics", () => ({ track: m.track }));
vi.mock("@/lib/share/render-card", () => ({ renderShareCard: m.render }));
vi.mock("@/lib/psychics-link", () => ({ usePsychicsHref: () => "/en#psychics" }));
// The sharer's invite (growth: referrals) needs a session; share-result.test.tsx covers it.
vi.mock("@/lib/referrals/use-share-invite", () => ({ useShareInvite: () => null }));
// The advisor strip (growth: booking) reads the live advisors; it has its own tests.
vi.mock("@/components/booking/advisor-strip", () => ({
  AdvisorStrip: ({ placement }: { placement: string }) => <div data-testid="advisor-strip" data-placement={placement} />,
}));
vi.mock("@/components/layout/public-frame", () => ({ PublicFrame: ({ children }: { children: React.ReactNode }) => <>{children}</> }));
vi.mock("@/components/astrology/place-combobox", () => ({ PlaceCombobox: () => null }));
vi.mock("@/components/calculators/report-email-dialog", () => ({ ReportEmailDialog: () => null }));
vi.mock("@/components/calculators/use-report-email-prompt", () => ({
  useReportEmailPrompt: (kind: string, active: boolean, paused = false) => {
    m.prompt(kind, active, paused);
    return { available: true, prechecked: false, configFailed: false, open: false, manual: false, setOpen: vi.fn(), openManually: vi.fn(), markSent: vi.fn() };
  },
}));

import { Calculator } from "@/components/calculators/calculator";
import { clearResults } from "@/components/calculators/calculate";

const fixture = natal as unknown as NatalResult;
const NATAL_HASH = "#v=1&d=1990-03-14&t=08:45&p=gn:2988507";

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function at(path: string, hash = "") {
  window.history.replaceState(null, "", `${path}${hash}`);
}

beforeAll(() => {
  stubResizeObserver();
  Element.prototype.hasPointerCapture ??= () => false;
  Element.prototype.setPointerCapture ??= () => undefined;
  Element.prototype.releasePointerCapture ??= () => undefined;
  Element.prototype.scrollIntoView ??= () => undefined;
});
beforeEach(() => {
  clearResults();
  for (const fn of Object.values(m)) fn.mockReset();
  m.render.mockResolvedValue(new Blob(["png"], { type: "image/png" }));
  window.scrollTo = vi.fn() as unknown as typeof window.scrollTo;
});
afterEach(cleanup);

async function chooseHouseSystem(u: ReturnType<typeof userEvent.setup>, name: string) {
  await u.click(screen.getByTestId("house-system-select"));
  await u.click(await screen.findByRole("option", { name }));
}

describe("natal chart calculator", () => {
  it("gives a visitor the full chart: big three, explorer, numbers computed locally, spread and the advisor CTA", async () => {
    m.natal.mockResolvedValue(fixture);
    at("/en/calculators/natal-chart", NATAL_HASH);
    renderWithI18n(<Calculator kind="natal-chart" />);

    expect(await screen.findByTestId("big-three")).toBeTruthy();
    expect(m.natal).toHaveBeenCalledTimes(1);
    expect(m.natal.mock.calls[0][0]).toEqual({
      birth_date: "1990-03-14",
      birth_time: "08:45",
      place_id: "gn:2988507",
      house_system: "placidus",
      include: ["tarot", "aspects"],
    });
    expect(screen.getByTestId("big-three").textContent).toBe("Sun in Pisces · Moon in Libra · Taurus rising");
    expect(screen.getByTestId("natal-wheel")).toBeTruthy();
    expect(screen.getByTestId("planet-row-Moon")).toBeTruthy();
    expect(screen.getByTestId("chart-balance")).toBeTruthy();

    const cycles = personalCycles("1990-03-14", localIsoDate());
    const numbers = within(screen.getByTestId("my-numbers")).getAllByRole("definition").map((d) => d.textContent);
    expect(numbers).toEqual([String(lifePathReduction("1990-03-14").number), String(cycles.personalYear), String(cycles.personalMonth)]);

    expect(screen.getByTestId("tarot-spread")).toBeTruthy();
    const cta = screen.getByTestId("advisor-cta");
    expect(within(cta).getByRole("heading", { name: "Talk to an advisor about your chart" })).toBeTruthy();
    expect(within(cta).getByRole("link", { name: "Browse all advisors" }).getAttribute("href")).toBe("/en#psychics");
    // Share next to the date and near the CTA; the e-mailed report stays.
    expect(screen.getAllByTestId("share-result-button")).toHaveLength(2);
    expect(screen.getByTestId("email-report-button")).toBeTruthy();
  });

  it("recalculates in place when the house system changes, keeps the chart meanwhile and caches each system", async () => {
    const u = userEvent.setup();
    m.natal.mockResolvedValueOnce(fixture);
    at("/en/calculators/natal-chart", NATAL_HASH);
    renderWithI18n(<Calculator kind="natal-chart" />);
    await screen.findByTestId("natal-wheel");

    const next = deferred<NatalResult>();
    m.natal.mockReturnValueOnce(next.promise);
    await chooseHouseSystem(u, "Whole Sign");
    expect(window.location.hash).toBe("#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507&h=whole_sign");
    await waitFor(() => expect(m.natal).toHaveBeenCalledTimes(2));
    expect(m.natal.mock.calls[1][0]).toMatchObject({ house_system: "whole_sign" });
    // The previous chart stays on screen while the new one loads.
    expect(screen.getByTestId("natal-wheel")).toBeTruthy();
    expect(screen.getByText("Updating…")).toBeTruthy();
    expect(screen.getByTestId("calculator-result").getAttribute("aria-busy")).toBe("true");

    await act(async () => next.resolve({ ...fixture, chart: { ...fixture.chart, house_system: "whole_sign", house_system_used: "whole_sign" } }));
    await waitFor(() => expect(screen.queryByText("Updating…")).toBeNull());
    expect(screen.getByText("Houses: Whole Sign.")).toBeTruthy();
    expect(screen.getByTestId("calculator-result").getAttribute("aria-busy")).toBe("false");

    // Back to Placidus: answered from the cache, no request.
    await chooseHouseSystem(u, "Placidus");
    expect(window.location.hash).toBe("#v=1&d=1990-03-14&t=08%3A45&p=gn%3A2988507");
    await waitFor(() => expect(screen.getByText("Houses: Placidus.")).toBeTruthy());
    expect(m.natal).toHaveBeenCalledTimes(2);
  });

  it("shows an inline error with a retry when the new house system cannot be calculated", async () => {
    const u = userEvent.setup();
    m.natal.mockResolvedValueOnce(fixture);
    at("/en/calculators/natal-chart", NATAL_HASH);
    renderWithI18n(<Calculator kind="natal-chart" />);
    await screen.findByTestId("natal-wheel");

    m.natal.mockRejectedValueOnce(new ApiError({ code: "network_error", message: "x" }));
    await chooseHouseSystem(u, "Equal");
    const alert = await screen.findByTestId("chart-update-error");
    expect(alert.textContent).toContain("We could not update your houses.");
    expect(screen.getByTestId("natal-wheel")).toBeTruthy();
    expect(screen.queryByTestId("calculator-form")).toBeNull();

    m.natal.mockResolvedValueOnce({ ...fixture, chart: { ...fixture.chart, house_system: "equal", house_system_used: "equal" } });
    await u.click(screen.getByTestId("chart-update-retry"));
    await waitFor(() => expect(screen.queryByTestId("chart-update-error")).toBeNull());
    expect(await screen.findByText("Houses: Equal.")).toBeTruthy();
    expect(m.natal).toHaveBeenCalledTimes(3);
  });

  it("names both Sun signs of a cusp day without a birth time, says why and leads to the form", async () => {
    const u = userEvent.setup();
    const chart = {
      ...fixture.chart,
      time_known: false,
      has_location: false,
      birth_date: "1990-02-19",
      birth_time: undefined,
      place: undefined,
      rising_sign: undefined,
      sun_sign: "Pisces",
      sun_sign_uncertain: true,
      sun_sign_alternatives: ["Aquarius", "Pisces"],
      moon_sign_uncertain: false,
    };
    m.natal.mockResolvedValue({ ...fixture, chart });
    at("/en/calculators/natal-chart", "#v=1&d=1990-02-19");
    renderWithI18n(<Calculator kind="natal-chart" />);
    expect((await screen.findByTestId("big-three")).textContent).toBe("Sun in Aquarius or Pisces · Moon in Libra");
    const note = screen.getByTestId("sun-uncertain");
    expect(note.textContent).toContain("You were born on a cusp: the Sun moved from Aquarius into Pisces that day.");
    expect(note.textContent).toContain("Your Sun sign needs your birth time and place.");
    await u.click(within(note).getByRole("button", { name: "Add birth time and place" }));
    const form = await screen.findByTestId("calculator-form");
    expect((form.querySelector('input[name="birth.year"]') as HTMLInputElement).value).toBe("1990");
    expect(window.location.hash).toBe("");
  });

  it("does not show the cusp note when the Sun sign is certain", async () => {
    m.natal.mockResolvedValue(fixture);
    at("/en/calculators/natal-chart", NATAL_HASH);
    renderWithI18n(<Calculator kind="natal-chart" />);
    await screen.findByTestId("big-three");
    expect(screen.queryByTestId("sun-uncertain")).toBeNull();
  });
});

describe("the sample /results routes", () => {
  it("'Change birth date' on the sample opens an empty form, and the visitor's own result follows", async () => {
    const u = userEvent.setup();
    at("/en/calculators/tarot-birth-card/results");
    const { container } = renderWithI18n(<Calculator kind="tarot-birth-card" showResults />);
    await screen.findByTestId("tarot-headline");
    await u.click(screen.getByTestId("change-birth-date"));
    const form = await screen.findByTestId("calculator-form");
    expect(screen.queryByTestId("calculator-result")).toBeNull();
    expect((form.querySelector('input[name="birth.year"]') as HTMLInputElement).value).toBe("");
    expect(window.location.pathname).toBe("/en/calculators/tarot-birth-card/results");

    fireEvent.change(container.querySelector('input[name="birth.day"]')!, { target: { value: "29" } });
    fireEvent.change(container.querySelector('input[name="birth.month"]')!, { target: { value: "11" } });
    fireEvent.change(container.querySelector('input[name="birth.year"]')!, { target: { value: "1990" } });
    fireEvent.click(screen.getByTestId("calculator-submit"));
    expect((await screen.findByTestId("tarot-headline")).textContent).toBe("Your tarot birth card is The Hierophant");
    expect(window.location.hash).toBe("#v=1&d=1990-11-29");
    expect(screen.queryByText("Sample date from the design. Change the birth date to calculate your own result.")).toBeNull();
    expect(screen.getAllByTestId("share-result-button")).toHaveLength(2);

    // Changing their own date on /results: the form again (with their date), not the sample.
    await u.click(screen.getByTestId("change-birth-date"));
    const again = await screen.findByTestId("calculator-form");
    expect((again.querySelector('input[name="birth.year"]') as HTMLInputElement).value).toBe("1990");
    expect(screen.queryByTestId("tarot-headline")).toBeNull();
  });

  it("'Add birth time and place' under the natal sample opens the form", async () => {
    const u = userEvent.setup();
    m.natal.mockResolvedValue({ ...fixture, chart: { ...fixture.chart, time_known: false, rising_sign: undefined, birth_date: "1945-04-30" } });
    at("/en/calculators/natal-chart/results");
    renderWithI18n(<Calculator kind="natal-chart" showResults />);
    await screen.findByTestId("natal-wheel");
    await u.click(screen.getByTestId("add-birth-time"));
    expect(await screen.findByTestId("calculator-form")).toBeTruthy();
    expect(screen.queryByTestId("natal-wheel")).toBeNull();
  });
});

describe("path of life calculator", () => {
  it("closes on the shared advisor CTA (clear link, share button, the birth data notice)", async () => {
    at("/en/calculators/path-of-life", "#v=1&d=1990-11-29");
    renderWithI18n(<Calculator kind="path-of-life" />);
    const cta = await screen.findByTestId("advisor-cta");
    expect(within(cta).getByRole("heading", { level: 2 }).textContent).toBe("Get a deeper understanding of what a life path really means and the alignment that comes with it");
    expect(within(cta).getByRole("link", { name: "Browse all advisors" }).getAttribute("href")).toBe("/en#psychics");
    expect(within(cta).getByTestId("share-result-button")).toBeTruthy();
    expect(within(cta).getByTestId("advisor-strip").dataset.placement).toBe("calculator_path_of_life");
    expect(cta.className).toContain("calc-paper");
    expect(within(cta).queryByText("Click here!")).toBeNull();
  });

  it("holds the 'Save your reading' prompt back while the share dialog is open", async () => {
    const u = userEvent.setup();
    at("/en/calculators/path-of-life", "#v=1&d=1990-11-29");
    renderWithI18n(<Calculator kind="path-of-life" />);
    await screen.findByTestId("life-path-numbers");
    expect(m.prompt).toHaveBeenLastCalledWith("path-of-life", true, false);
    await u.click(screen.getAllByTestId("share-result-button")[0]);
    expect(await screen.findByTestId("share-dialog")).toBeTruthy();
    expect(m.prompt).toHaveBeenLastCalledWith("path-of-life", true, true);
    await u.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByTestId("share-dialog")).toBeNull());
    expect(m.prompt).toHaveBeenLastCalledWith("path-of-life", true, false);
  });
});

describe("astrology calculator", () => {
  it("offers the free natal chart with the same birth details instead of a sign-up wall", async () => {
    m.sunSign.mockResolvedValue({ sun_sign: "Taurus", uncertain: false, element: "Earth", modality: "Fixed" });
    at("/en/calculators/astrology", "#v=1&d=1990-05-02&t=08:45&p=gn:2988507");
    renderWithI18n(<Calculator kind="astrology" />);
    const link = await screen.findByTestId("full-chart-link");
    expect(link.textContent).toBe("See your full birth chart — free");
    expect(link.getAttribute("href")).toBe("/en/calculators/natal-chart#v=1&d=1990-05-02&t=08%3A45&p=gn%3A2988507");
    expect(screen.queryByText("Sign in / up")).toBeNull();
    expect(screen.getAllByTestId("share-result-button").length).toBeGreaterThan(0);
  });
});

describe("tarot birth card calculator", () => {
  it("calculates in the browser from the date, writes the fragment and shows the card, the sums and the CTA", async () => {
    at("/en/calculators/tarot-birth-card");
    const { container } = renderWithI18n(<Calculator kind="tarot-birth-card" />);
    const form = screen.getByTestId("calculator-form");
    expect(within(form).getByText("Your birth date is used only in your browser to find your card. It is never sent to us.")).toBeTruthy();
    expect(m.track).toHaveBeenCalledWith("calculator_started", { kind: "tarot-birth-card" });

    fireEvent.change(container.querySelector('input[name="birth.day"]')!, { target: { value: "29" } });
    fireEvent.change(container.querySelector('input[name="birth.month"]')!, { target: { value: "11" } });
    fireEvent.change(container.querySelector('input[name="birth.year"]')!, { target: { value: "1990" } });
    fireEvent.click(screen.getByTestId("calculator-submit"));

    expect(await screen.findByTestId("tarot-headline")).toBeTruthy();
    expect(window.location.hash).toBe("#v=1&d=1990-11-29");
    expect(screen.getByTestId("tarot-headline").textContent).toBe("Your tarot birth card is The Hierophant");
    expect(screen.getByTestId("tarot-steps").textContent).toBe("11 + 29 + 1990 = 2030 → 2+0+3+0 = 5");
    expect(screen.getByRole("heading", { level: 1 }).textContent).toBe("Tarot birth card calculated for:");
    const card = screen.getByTestId("birth-card");
    expect(card.getAttribute("data-role")).toBe("Your birth card");
    expect(within(card).getByRole("img", { name: "The Hierophant" }).getAttribute("src")).toBe("https://assets.staryield.net/assets/tarot/the-hierophant.png");
    expect(within(card).getByTestId("birth-card-numeral").textContent).toBe("V");
    expect(screen.getByTestId("birth-card-meaning").textContent).toContain("Tradition, teaching, belonging");
    // Its constellation: the other chain that ends in The Hierophant.
    expect([...screen.getByTestId("tarot-constellation").querySelectorAll("[data-chain]")].map((li) => li.getAttribute("data-chain"))).toEqual(["14/5"]);
    expect(within(screen.getByTestId("advisor-cta")).getByRole("heading", { name: "Talk to an advisor about your card" })).toBeTruthy();
    // No e-mail report for this calculator: no button and no "Save your reading" prompt.
    expect(screen.queryByTestId("email-report-button")).toBeNull();
    expect(m.prompt).toHaveBeenLastCalledWith("tarot-birth-card", false, false);
    expect(screen.getAllByTestId("share-result-button")).toHaveLength(2);
    expect(m.track).toHaveBeenCalledWith("calculator_completed", { kind: "tarot-birth-card" });
    expect(m.natal).not.toHaveBeenCalled();
  });

  it("restores a two-card result from a shared link, personality to soul, with the constellation", async () => {
    at("/en/calculators/tarot-birth-card", "#v=1&d=1987-06-15");
    renderWithI18n(<Calculator kind="tarot-birth-card" />);
    const cards = await screen.findAllByTestId("birth-card");
    expect(cards.map((c) => [c.getAttribute("data-card"), c.getAttribute("data-role")])).toEqual([
      ["wheel-of-fortune", "Personality card"],
      ["the-magician", "Soul card"],
    ]);
    expect(cards.map((c) => within(c).getByTestId("birth-card-numeral").textContent)).toEqual(["X", "I"]);
    expect(screen.getByTestId("tarot-headline").textContent).toBe("Your tarot birth card is Wheel of Fortune");
    expect(screen.getByTestId("tarot-chain").textContent).toBe("10 Wheel of Fortune → 1 The Magician");
    expect(screen.getByText("Others with The Magician as soul card:")).toBeTruthy();
    const chains = [...screen.getByTestId("tarot-constellation").querySelectorAll("[data-chain]")];
    expect(chains.map((li) => li.getAttribute("data-chain"))).toEqual(["19/10/1"]);
    expect(chains[0].textContent).toContain("The Sun → Wheel of Fortune → The Magician");
  });

  it("numbers The Fool 0 and pairs it with The Emperor", async () => {
    at("/en/calculators/tarot-birth-card", "#v=1&d=1940-07-10");
    renderWithI18n(<Calculator kind="tarot-birth-card" />);
    const cards = await screen.findAllByTestId("birth-card");
    expect(cards.map((c) => within(c).getByTestId("birth-card-numeral").textContent)).toEqual(["0", "IV"]);
    expect([...screen.getByTestId("tarot-constellation").querySelectorAll("[data-chain]")].map((li) => li.getAttribute("data-chain"))).toEqual(["4", "13/4"]);
  });

  it("refuses a link with keys this calculator does not use", () => {
    at("/en/calculators/tarot-birth-card", "#v=1&d=1990-11-29&t=08:45");
    renderWithI18n(<Calculator kind="tarot-birth-card" />);
    expect(screen.getByTestId("calculator-form")).toBeTruthy();
    expect(screen.getByText("This link's birth details are incomplete or invalid. Please check them below.")).toBeTruthy();
  });

  it("shows the design's sample on /results without a share button (it is not the visitor's card)", async () => {
    at("/en/calculators/tarot-birth-card/results");
    renderWithI18n(<Calculator kind="tarot-birth-card" showResults />);
    expect(await screen.findByTestId("tarot-headline")).toBeTruthy();
    expect(screen.getByText("Sample date from the design. Change the birth date to calculate your own result.")).toBeTruthy();
    expect(screen.queryByTestId("share-result-button")).toBeNull();
  });
});
