// @vitest-environment jsdom
import type * as React from "react";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { ApiError } from "@/lib/api/errors";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const api = vi.hoisted(() => ({ send: vi.fn() }));
const session = vi.hoisted(() => ({ current: {} as Record<string, unknown> }));
vi.mock("@/lib/api/calculator-report", () => ({ reportEmailApi: api }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => session.current }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/booking/advisor-strip", () => ({
  AdvisorStrip: ({ heading, placement }: { heading: string; placement: string }) => (
    <section data-testid="advisor-strip-stub" data-placement={placement}>
      <h3>{heading}</h3>
    </section>
  ),
}));
vi.mock("@/lib/api/astrology", async (importOriginal) => ({ ...(await importOriginal<typeof import("@/lib/api/astrology")>()), browserTimeZone: () => "Europe/Paris" }));

import { ReportEmailDialog, reportEmailErrorCopy } from "@/components/calculators/report-email-dialog";

const NATAL = {
  birth: { day: "14", month: "03", year: "1990", time: "08:45" },
  timeUnknown: false,
  place: { id: "gn:2988507", label: "Paris, France" },
  houseSystem: "placidus",
} as never;
const LIFE = { birth: { day: "29", month: "11", year: "1990", time: "" } } as never;

function setup(props: Partial<React.ComponentProps<typeof ReportEmailDialog>> = {}) {
  const onOpenChange = vi.fn();
  const onSent = vi.fn();
  const view = renderWithI18n(
    <ReportEmailDialog kind="natal-chart" values={NATAL} open onOpenChange={onOpenChange} prechecked={false} onSent={onSent} manual {...props} />,
  );
  return { onOpenChange, onSent, ...view };
}
const email = () => screen.getByRole("textbox", { name: "Email" }) as HTMLInputElement;
const marketing = () => screen.getByRole("checkbox") as HTMLElement;
const submit = () => screen.getByRole("button", { name: /Send my report|Sending/ });
const apiError = (code: string, status: number, extra: Record<string, unknown> = {}) => new ApiError({ code, message: code, status, ...extra });

beforeAll(stubResizeObserver);
beforeEach(() => {
  session.current = { status: "anonymous", user: null };
  api.send.mockReset();
  api.send.mockResolvedValue({ status: "queued" });
  vi.spyOn(window.navigator, "onLine", "get").mockReturnValue(true);
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("ReportEmailDialog: the form", () => {
  it("says what it is for and has the three controls of the contract", () => {
    setup();
    expect(screen.getByRole("dialog", { name: "Save your reading" })).toBeTruthy();
    expect(email().type).toBe("email");
    expect(screen.getByText("Send me horoscopes, offers and news from Staryield by e-mail. You can unsubscribe at any time.")).toBeTruthy();
    expect(marketing().getAttribute("aria-checked")).toBe("false");
    expect(submit().textContent).toBe("Send my report");
    expect(screen.getByRole("link", { name: "Privacy Policy" }).getAttribute("href")).toBe("/en/privacy");
    expect(screen.getByRole("button", { name: "No thanks" })).toBeTruthy();
  });

  it("the footnote says the e-mail is used for news and offers only if the box is ticked", () => {
    setup();
    const note = screen.getByRole("link", { name: "Privacy Policy" }).closest("p")!;
    expect(note.textContent).toBe(
      "We use your e-mail address to send this report and, only if the box above is ticked, to send you news and offers. We keep your birth details only until the report is sent. Privacy Policy",
    );
    expect(note.textContent).not.toContain("only to send this report");
    // the consent evidence is untouched
    expect(screen.getByText("Send me horoscopes, offers and news from Staryield by e-mail. You can unsubscribe at any time.")).toBeTruthy();
  });

  it("starts with the box ticked only when the API said so (pre-ticked countries)", () => {
    setup({ prechecked: true });
    expect(marketing().getAttribute("aria-checked")).toBe("true");
  });

  it("follows a config that arrives late, until the visitor touches the box", async () => {
    const u = userEvent.setup();
    const { rerender } = setup({ prechecked: false });
    rerender(<ReportEmailDialog kind="natal-chart" values={NATAL} open onOpenChange={vi.fn()} prechecked />);
    await waitFor(() => expect(marketing().getAttribute("aria-checked")).toBe("true"));
    await u.click(marketing());
    expect(marketing().getAttribute("aria-checked")).toBe("false");
    rerender(<ReportEmailDialog kind="natal-chart" values={NATAL} open onOpenChange={vi.fn()} prechecked={false} />);
    rerender(<ReportEmailDialog kind="natal-chart" values={NATAL} open onOpenChange={vi.fn()} prechecked />);
    expect(marketing().getAttribute("aria-checked")).toBe("false"); // the visitor's choice stands
  });

  it("prefills the address of a signed-in visitor", () => {
    session.current = { status: "authenticated", user: { email: "me@example.com" } };
    setup();
    expect(email().value).toBe("me@example.com");
  });

  it("hides the honeypot from people: off-screen, out of the tab order, aria-hidden", () => {
    setup();
    const hp = screen.getByTestId("report-email-website");
    expect(hp.getAttribute("tabindex")).toBe("-1");
    expect(hp.getAttribute("autocomplete")).toBe("off");
    expect(hp.getAttribute("name")).not.toBe("website"); // browsers autofill fields called website
    expect(hp.closest("[aria-hidden='true']")?.className).toContain("report-honeypot");
  });

  it("has no accessibility violations", async () => {
    setup();
    expect(await axeViolations()).toEqual([]);
  });

  it("dismisses with 'No thanks'", async () => {
    const u = userEvent.setup();
    const { onOpenChange } = setup();
    await u.click(screen.getByRole("button", { name: "No thanks" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});

describe("ReportEmailDialog: sending", () => {
  it("validates the address before calling the API", async () => {
    const u = userEvent.setup();
    setup();
    await u.click(submit());
    expect(await screen.findByText("Enter your email address.")).toBeTruthy();
    await u.type(email(), "not-an-email");
    await u.click(submit());
    expect(await screen.findByText("Enter a valid email address.")).toBeTruthy();
    expect(email().getAttribute("aria-invalid")).toBe("true");
    expect(api.send).not.toHaveBeenCalled();
  });

  it("sends the calculator's own request and confirms with the address", async () => {
    const u = userEvent.setup();
    const { onSent } = setup();
    await u.type(email(), "  Jane@Example.com ");
    await u.click(submit());
    // (also announced in the live region: the visible confirmation is the dialog's)
    expect((await screen.findByTestId("report-email-success")).textContent).toContain("Your report is on its way to Jane@Example.com.");
    expect(api.send).toHaveBeenCalledTimes(1);
    expect(api.send).toHaveBeenCalledWith({
      kind: "natal-chart",
      input: { birth_date: "1990-03-14", birth_time: "08:45", place_id: "gn:2988507", house_system: "placidus", include: ["tarot"] },
      email: "Jane@Example.com",
      locale: "en",
      marketing_opt_in: false,
      marketing_prechecked: false,
      website: "",
    });
    expect(onSent).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("report-email-success").textContent).toContain("check your spam folder");
    expect(screen.queryByTestId("report-email-form")).toBeNull();
  });

  it("offers a next step after sending: advisors, and a free account to a signed-out visitor", async () => {
    window.history.pushState({}, "", "/en/calculators/natal-chart");
    const u = userEvent.setup();
    setup();
    expect(screen.queryByTestId("advisor-strip-stub")).toBeNull();
    await u.type(email(), "jane@example.com");
    await u.click(submit());
    const success = await screen.findByTestId("report-email-success");
    const strip = within(success).getByTestId("advisor-strip-stub");
    expect(strip.getAttribute("data-placement")).toBe("report_success");
    expect(within(strip).getByRole("heading", { name: "Ask an advisor about your results" })).toBeTruthy();
    expect(within(success).getByRole("link", { name: "Create a free account" }).getAttribute("href")).toBe(
      "/en/signup?next=%2Fen%2Fcalculators%2Fnatal-chart",
    );
    expect(within(success).getByTestId("report-email-close")).toBeTruthy();
  });

  it("offers no sign-up link to a signed-in visitor, but still the advisors", async () => {
    session.current = { status: "authenticated", user: { email: "me@example.com" } };
    const u = userEvent.setup();
    setup();
    await u.click(submit());
    const success = await screen.findByTestId("report-email-success");
    expect(within(success).getByTestId("advisor-strip-stub")).toBeTruthy();
    expect(screen.queryByRole("link", { name: "Create a free account" })).toBeNull();
  });

  it("closes from the success state", async () => {
    const u = userEvent.setup();
    const { onOpenChange } = setup();
    await u.type(email(), "jane@example.com");
    await u.click(submit());
    await u.click(await screen.findByTestId("report-email-close"));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("reports a pre-ticked box that was left alone as pre-ticked, and a box the visitor ticked as theirs", async () => {
    const u = userEvent.setup();
    setup({ prechecked: true });
    await u.type(email(), "us@example.com");
    await u.click(submit());
    await screen.findByTestId("report-email-success");
    expect(api.send).toHaveBeenLastCalledWith(expect.objectContaining({ marketing_opt_in: true, marketing_prechecked: true }));
    cleanup();

    // unticking and ticking again is the visitor's own choice
    setup({ prechecked: true });
    await u.type(email(), "us@example.com");
    await u.click(marketing());
    await u.click(marketing());
    await u.click(submit());
    await screen.findByTestId("report-email-success");
    expect(api.send).toHaveBeenLastCalledWith(expect.objectContaining({ marketing_opt_in: true, marketing_prechecked: false }));
    cleanup();

    // ticked by the visitor where the box starts unticked
    setup({ prechecked: false });
    await u.type(email(), "fr@example.com");
    await u.click(marketing());
    await u.click(submit());
    await screen.findByTestId("report-email-success");
    expect(api.send).toHaveBeenLastCalledWith(expect.objectContaining({ marketing_opt_in: true, marketing_prechecked: false }));
    cleanup();

    // unticked: no opt-in, whatever the default was
    setup({ prechecked: true });
    await u.type(email(), "us@example.com");
    await u.click(marketing());
    await u.click(submit());
    await screen.findByTestId("report-email-success");
    expect(api.send).toHaveBeenLastCalledWith(expect.objectContaining({ marketing_opt_in: false, marketing_prechecked: false }));
  });

  it("sends the honeypot as typed (a bot filling it is dropped by the API)", async () => {
    const u = userEvent.setup();
    setup();
    await u.type(email(), "bot@example.com");
    fireEvent.change(screen.getByTestId("report-email-website"), { target: { value: "https://spam.example" } });
    await u.click(submit());
    await screen.findByTestId("report-email-success");
    expect(api.send).toHaveBeenCalledWith(expect.objectContaining({ website: "https://spam.example" }));
  });

  it("sends the path of life's numerology request with the visitor's zone", async () => {
    const u = userEvent.setup();
    setup({ kind: "path-of-life", values: LIFE });
    await u.type(email(), "life@example.com");
    await u.click(submit());
    await screen.findByTestId("report-email-success");
    expect(api.send).toHaveBeenCalledWith(expect.objectContaining({ kind: "path-of-life", input: { birth_date: "1990-11-29", tz: "Europe/Paris" } }));
  });

  it("retries without the zone when the API does not know it", async () => {
    const u = userEvent.setup();
    api.send.mockRejectedValueOnce(apiError("validation_failed", 422, { details: { fields: { "input.tz": "Choose a valid time zone." } } }));
    setup({ kind: "path-of-life", values: LIFE });
    await u.type(email(), "life@example.com");
    await u.click(submit());
    await screen.findByTestId("report-email-success");
    expect(api.send).toHaveBeenCalledTimes(2);
    expect(api.send).toHaveBeenLastCalledWith(expect.objectContaining({ input: { birth_date: "1990-11-29" } }));
  });

  it("disables the button while sending so a double click sends once", async () => {
    const u = userEvent.setup();
    let release: (v: unknown) => void = () => undefined;
    api.send.mockReturnValue(new Promise((resolve) => (release = resolve)));
    setup();
    await u.type(email(), "jane@example.com");
    await u.click(submit());
    await waitFor(() => expect((submit() as HTMLButtonElement).disabled).toBe(true));
    expect(submit().textContent).toBe("Sending…");
    expect(submit().getAttribute("aria-busy")).toBe("true");
    await u.click(submit());
    expect(api.send).toHaveBeenCalledTimes(1);
    release({ status: "queued" });
    await screen.findByTestId("report-email-success");
  });
});

describe("ReportEmailDialog: errors", () => {
  async function fillAndSubmit() {
    const u = userEvent.setup();
    await u.type(email(), "jane@example.com");
    await u.click(submit());
    return u;
  }

  it("puts an e-mail validation error from the API on the field", async () => {
    api.send.mockRejectedValue(apiError("validation_failed", 422, { details: { fields: { email: "Enter a valid email address." } } }));
    setup();
    await fillAndSubmit();
    expect(await screen.findByText("Enter a valid email address.")).toBeTruthy();
    expect(screen.queryByTestId("report-email-error")).toBeNull();
  });

  it("says 'try again later' on the limit, without a retry button", async () => {
    api.send.mockRejectedValue(apiError("rate_limited", 429, { retryAfter: 3600 }));
    setup();
    await fillAndSubmit();
    const alert = await screen.findByTestId("report-email-error");
    expect(alert.textContent).toContain("Too many requests. Please try again later.");
    expect(screen.queryByTestId("report-email-retry")).toBeNull();
    expect((submit() as HTMLButtonElement).disabled).toBe(false);
  });

  it("offers a retry after a network failure, and a retry succeeds", async () => {
    api.send.mockRejectedValueOnce(apiError("network_error", 0));
    setup();
    const u = await fillAndSubmit();
    const alert = await screen.findByTestId("report-email-error");
    expect(alert.textContent).toContain("We couldn't reach Staryield. Please try again in a moment.");
    await u.click(screen.getByTestId("report-email-retry"));
    expect(await screen.findByTestId("report-email-success")).toBeTruthy();
    expect(api.send).toHaveBeenCalledTimes(2);
  });

  it("says offline when the browser is offline", async () => {
    vi.spyOn(window.navigator, "onLine", "get").mockReturnValue(false);
    api.send.mockRejectedValue(apiError("network_error", 0));
    setup();
    await fillAndSubmit();
    expect((await screen.findByTestId("report-email-error")).textContent).toContain("You appear to be offline.");
  });

  it("says the report is unavailable when it was switched off, with no retry", async () => {
    api.send.mockRejectedValue(apiError("feature_unavailable", 503));
    setup();
    await fillAndSubmit();
    const alert = await screen.findByTestId("report-email-error");
    expect(alert.textContent).toContain("The e-mail report isn't available right now.");
    expect(screen.queryByTestId("report-email-retry")).toBeNull();
  });

  it("explains an input the API refused", async () => {
    api.send.mockRejectedValue(apiError("validation_failed", 422, { details: { fields: { "input.birth_date": "Enter a valid birth date." } } }));
    setup();
    await fillAndSubmit();
    expect((await screen.findByTestId("report-email-error")).textContent).toContain("We couldn't prepare this report.");
    expect(screen.queryByTestId("report-email-retry")).toBeNull();
  });

  it("retries after a server error", async () => {
    api.send.mockRejectedValueOnce(apiError("internal_error", 500));
    setup();
    await fillAndSubmit();
    expect((await screen.findByTestId("report-email-error")).textContent).toContain("Something went wrong on our side.");
    expect(screen.getByTestId("report-email-retry")).toBeTruthy();
  });

  it("has copy for every code it handles", () => {
    expect(reportEmailErrorCopy(apiError("rate_limited", 429))).toBe("Too many requests. Please try again later.");
    expect(reportEmailErrorCopy(apiError("timeout", 504))).toBe("The request took too long. Please try again.");
  });
});

describe("ReportEmailDialog: opening", () => {
  it("opened by itself it does not steal focus into the e-mail field (no keyboard pop-up on phones)", async () => {
    setup({ manual: false });
    await waitFor(() => expect(screen.getByTestId("report-email-dialog")).toBeTruthy());
    expect(document.activeElement).not.toBe(email());
    expect(document.activeElement).toBe(screen.getByTestId("report-email-dialog"));
  });

  it("opened by the button it starts in the e-mail field", async () => {
    setup({ manual: true });
    await waitFor(() => expect(document.activeElement).toBe(email()));
  });

  it("starts fresh every time it opens", async () => {
    const u = userEvent.setup();
    const { rerender } = setup();
    await u.type(email(), "jane@example.com");
    await u.click(submit());
    await screen.findByTestId("report-email-success");
    rerender(<ReportEmailDialog kind="natal-chart" values={NATAL} open={false} onOpenChange={vi.fn()} prechecked={false} />);
    await waitFor(() => expect(screen.queryByTestId("report-email-dialog")).toBeNull());
    rerender(<ReportEmailDialog kind="natal-chart" values={NATAL} open onOpenChange={vi.fn()} prechecked={false} manual />);
    expect(await screen.findByTestId("report-email-form")).toBeTruthy();
    expect(email().value).toBe("");
  });
});
