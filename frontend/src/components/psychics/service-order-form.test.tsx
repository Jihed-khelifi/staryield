// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type * as React from "react";

import { ApiError } from "@/lib/api/errors";
import type { Service } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";
import { ServiceOrderForm } from "@/components/psychics/service-order-form";

const order = vi.hoisted(() => vi.fn());
const toast = vi.hoisted(() => ({ show: vi.fn(), showError: vi.fn() }));
const setWallet = vi.hoisted(() => vi.fn());
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/lib/analytics", () => ({ track: vi.fn() }));
vi.mock("@/lib/api/endpoints", () => ({ requestsApi: { order } }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => toast }));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => ({ status: "authenticated", wallet: { balance: 900 }, setWallet }),
}));

const service: Service = {
  id: "svc1",
  name: "Love reading",
  subtitle: "Written",
  description: "A detailed written reading.",
  price_credits: 600,
  active: true,
};

beforeEach(() => {
  vi.useFakeTimers({ toFake: ["Date"] });
  vi.setSystemTime(new Date(2026, 10, 5, 10, 0));
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
  vi.clearAllMocks();
});

function setup() {
  renderWithI18n(<ServiceOrderForm service={service} advisorName="Luna" slug="luna" returnHere="%2Fen%2Fpsychics%2Fluna" />);
  const form = screen.getByTestId("order-form");
  return {
    form,
    u: userEvent.setup(),
    date: within(form).getByLabelText("Preferred date") as HTMLInputElement,
    time: within(form).getByLabelText("Preferred time") as HTMLInputElement,
    send: () => within(form).getByRole("button", { name: "Request a session" }),
  };
}

describe("ServiceOrderForm", () => {
  it("bounds the date picker to what the API accepts", () => {
    const { date } = setup();
    expect(date.min).toBe("2026-11-05");
    expect(date.max).toBe("2027-11-04");
  });

  it("needs a date for a time and refuses past dates", async () => {
    const { form, date, time, send, u } = setup();
    fireEvent.change(time, { target: { value: "18:30" } });
    await u.click(send());
    expect(await within(form).findByText("Choose a date for this time.")).toBeTruthy();
    fireEvent.change(date, { target: { value: "2026-11-04" } });
    expect(await within(form).findByText("Choose today or a later date.")).toBeTruthy();
    fireEvent.change(date, { target: { value: "2026-11-06" } });
    await waitFor(() => expect(within(form).queryByText("Choose a date for this time.")).toBeNull());
    expect(order).not.toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
  });

  it("limits the note to 2,000 characters", async () => {
    const { form, send, u } = setup();
    await u.click(within(form).getByRole("textbox", { name: "What would you like to explore?" }));
    await u.paste("x".repeat(2001));
    await u.click(send());
    expect(await within(form).findByText("Use at most 2000 characters.")).toBeTruthy();
    expect(order).not.toHaveBeenCalled();
  });

  it("orders with the preferred instant and a stable idempotency key", async () => {
    const { form, date, time, send, u } = setup();
    order
      .mockRejectedValueOnce(new ApiError({ code: "insufficient_credits", message: "poor", status: 402 }))
      .mockResolvedValueOnce({ id: "req1", price_credits: 600, wallet: { balance: 300 } });
    await u.type(within(form).getByRole("textbox", { name: "What would you like to explore?" }), "  Career change  ");
    fireEvent.change(date, { target: { value: "2026-11-20" } });
    fireEvent.change(time, { target: { value: "18:30" } });
    await u.click(send());
    const alert = await within(form).findByRole("alert");
    expect(alert.textContent).toContain("You do not have enough credits for this.");
    expect(within(alert).getByRole("link", { name: "Top up credits" }).getAttribute("href")).toBe(
      "/en/credits?next=%2Fen%2Fpsychics%2Fluna"
    );

    await u.click(send());
    await waitFor(() => expect(order).toHaveBeenCalledTimes(2));
    const [first, second] = order.mock.calls.map((call) => call[0]);
    expect(first).toEqual({
      service_id: "svc1",
      note: "Career change",
      preferred_at: new Date(2026, 10, 20, 18, 30).toISOString(),
      idempotency_key: expect.any(String),
    });
    expect(second.idempotency_key).toBe(first.idempotency_key);
    expect(await within(form).findByText("Your request was sent. 600 credits are held until Luna delivers it.")).toBeTruthy();
    expect(setWallet).toHaveBeenCalledWith({ balance: 300 });
    expect(toast.show).toHaveBeenCalledWith({ tone: "success", message: "Request sent to Luna." });
    expect(within(form).queryByRole("alert")).toBeNull();
  });

  it("puts a server date error on the date field", async () => {
    const { form, date, send, u } = setup();
    order.mockRejectedValue(
      new ApiError({ code: "validation_failed", message: "bad", status: 422, details: { fields: { preferred_at: "Choose a date within the next year." } } })
    );
    fireEvent.change(date, { target: { value: "2026-12-01" } });
    await u.click(send());
    expect(await within(form).findByText("Choose a date within the next year.")).toBeTruthy();
    expect(date.getAttribute("aria-invalid")).toBe("true");
  });
});
