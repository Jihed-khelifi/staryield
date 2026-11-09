// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { ApiError } from "@/lib/api/errors";
import type { PsychicAnalytics, Withdrawal } from "@/lib/api/types";
import { axeViolations, renderWithI18n } from "@/test/render";
import { Withdrawals } from "@/components/analytics/withdrawals";

const api = vi.hoisted(() => ({ list: vi.fn(), request: vi.fn(), cancel: vi.fn() }));
const toast = vi.hoisted(() => ({ show: vi.fn(), showError: vi.fn() }));
vi.mock("@/lib/api/endpoints", () => ({ withdrawalsApi: api }));
vi.mock("@/components/feedback/toast-provider", () => ({ useToast: () => toast }));

const payout: PsychicAnalytics["payout"] = {
  available_credits: 5000,
  reserved_credits: 0,
  paid_credits: 1200,
  lifetime_net_credits: 6200,
  cents_per_credit: 2,
  currency: "USD",
  min_withdrawal_credits: 2500,
};
const created: Withdrawal = {
  id: "w1",
  amount_credits: 3000,
  amount_cents: 6000,
  currency: "USD",
  method: "bank_transfer",
  destination_hint: "•••• 0189",
  status: "pending",
  note: "",
  requested_at: "2026-11-05T10:00:00Z",
};

beforeEach(() => api.list.mockResolvedValue({ items: [], next_cursor: "" }));
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

function setup(p: PsychicAnalytics["payout"] | null = payout) {
  const onChanged = vi.fn();
  renderWithI18n(<Withdrawals payout={p} onChanged={onChanged} />);
  return { onChanged, form: screen.getByTestId("withdrawal-form"), u: userEvent.setup() };
}

describe("Withdrawals form", () => {
  it("checks the amount against the payout limits", async () => {
    const { form, u } = setup();
    const amount = within(form).getByRole("textbox", { name: "Amount (credits)" });
    await u.type(amount, "100");
    await u.tab();
    expect(await within(form).findByText("The minimum withdrawal is 2,500 credits.")).toBeTruthy();
    await u.clear(amount);
    await u.type(amount, "9000");
    expect(await within(form).findByText("You can withdraw at most 5,000 credits.")).toBeTruthy();
    await u.click(within(form).getByRole("button", { name: "Max" }));
    expect((amount as HTMLInputElement).value).toBe("5000");
    await waitFor(() => expect(amount.getAttribute("aria-invalid")).toBe("false"));
    expect(await axeViolations()).toEqual([]);
  });

  it("validates the destination for the chosen method", async () => {
    const { form, u } = setup();
    await u.type(within(form).getByRole("textbox", { name: "IBAN or account number" }), "me@example.com");
    await u.tab();
    expect(await within(form).findByText("Enter a valid IBAN or account number.")).toBeTruthy();
    await u.click(within(form).getByRole("radio", { name: "PayPal" }));
    expect(within(form).getByLabelText("PayPal email")).toBeTruthy();
    await waitFor(() => expect(within(form).queryByText("Enter a valid IBAN or account number.")).toBeNull());
  });

  it("requests a withdrawal with one idempotency key across retries", async () => {
    const { form, onChanged, u } = setup();
    const online = vi.spyOn(window.navigator, "onLine", "get").mockReturnValue(false); // the offline copy
    api.request
      .mockRejectedValueOnce(new ApiError({ code: "network_error", message: "offline", status: 0 }))
      .mockResolvedValueOnce(created);
    await u.type(within(form).getByRole("textbox", { name: "Amount (credits)" }), "3000");
    await u.type(within(form).getByRole("textbox", { name: "IBAN or account number" }), "  FR76 3000 6000 0112 3456 7890 189 ");
    await u.click(within(form).getByRole("button", { name: "Request withdrawal" }));
    expect((await within(form).findByRole("alert")).textContent).toContain("You appear to be offline.");
    online.mockRestore();
    await u.click(within(form).getByRole("button", { name: "Request withdrawal" }));
    await waitFor(() => expect(onChanged).toHaveBeenCalled());
    const [first, second] = api.request.mock.calls.map((call) => call[0]);
    expect(first).toEqual({
      amount_credits: 3000,
      method: "bank_transfer",
      destination: "FR76 3000 6000 0112 3456 7890 189",
      idempotency_key: expect.any(String),
    });
    expect(second.idempotency_key).toBe(first.idempotency_key);
    expect(toast.show).toHaveBeenCalledWith(expect.objectContaining({ tone: "success" }));
    expect(await screen.findByTestId("withdrawal-history")).toBeTruthy();
    expect(within(form).getByRole("status").textContent).toContain("You already have a withdrawal in progress.");
  });

  it("maps server field names onto the form", async () => {
    const { form, u } = setup();
    api.request.mockRejectedValue(
      new ApiError({
        code: "validation_failed",
        message: "bad",
        status: 422,
        details: { fields: { amount_credits: "That is more than your available balance." } },
      })
    );
    await u.type(within(form).getByRole("textbox", { name: "Amount (credits)" }), "3000");
    await u.type(within(form).getByRole("textbox", { name: "IBAN or account number" }), "FR7630006000011234567890189");
    await u.click(within(form).getByRole("button", { name: "Request withdrawal" }));
    expect(await within(form).findByText("That is more than your available balance.")).toBeTruthy();
    expect(within(form).getByRole("textbox", { name: "Amount (credits)" }).getAttribute("aria-invalid")).toBe("true");
  });

  it("is disabled until the payout snapshot loads", () => {
    const { form } = setup(null);
    expect(within(form).getByRole("button", { name: "Request withdrawal" }).closest("fieldset")?.disabled).toBe(true);
  });
});
