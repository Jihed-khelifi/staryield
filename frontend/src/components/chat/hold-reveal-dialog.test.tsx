// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, screen } from "@testing-library/react";

import { axeViolations, renderWithI18n } from "@/test/render";
import { HoldRevealDialog } from "@/components/chat/hold-reveal-dialog";

afterEach(cleanup);

const confirm = (over: Partial<{ holdId: string; price: number; count: number; error: string | null }> = {}) => ({ holdId: "h1", price: 40, count: 3, error: null, ...over });
const open = (c = confirm(), props: Partial<Parameters<typeof HoldRevealDialog>[0]> = {}) => {
  const onConfirm = vi.fn();
  const onCancel = vi.fn();
  renderWithI18n(<HoldRevealDialog confirm={c} name="Theo" busy={false} onConfirm={onConfirm} onCancel={onCancel} {...props} />);
  return { onConfirm, onCancel };
};

describe("HoldRevealDialog", () => {
  it("asks the contract question with the count, the name and the price", () => {
    open();
    expect(screen.getByRole("alertdialog")).toBeTruthy();
    expect(screen.getByTestId("hold-reveal-question").textContent).toBe("Reveal 3 hidden messages from Theo for 40 credits?");
    expect(screen.getByTestId("hold-reveal-confirm").textContent).toBe("Reveal for 40 credits");
  });

  it("uses the singular for one message", () => {
    open(confirm({ count: 1 }));
    expect(screen.getByTestId("hold-reveal-question").textContent).toBe("Reveal 1 hidden message from Theo for 40 credits?");
  });

  it("confirm calls onConfirm and does not close by itself; Not now cancels", () => {
    const { onConfirm, onCancel } = open();
    fireEvent.click(screen.getByTestId("hold-reveal-confirm"));
    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onCancel).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Not now" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("shows a failed attempt's reason with 'Try again'", () => {
    const { onConfirm } = open(confirm({ error: "We couldn't reach Staryield." }));
    expect(screen.getByTestId("hold-reveal-error").textContent).toBe("We couldn't reach Staryield.");
    const retry = screen.getByTestId("hold-reveal-confirm");
    expect(retry.textContent).toBe("Try again");
    fireEvent.click(retry);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("locks both buttons while the request runs", () => {
    open(confirm(), { busy: true });
    expect((screen.getByTestId("hold-reveal-confirm") as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole("button", { name: "Not now" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("renders nothing without a confirm", () => {
    renderWithI18n(<HoldRevealDialog confirm={null} name="Theo" busy={false} onConfirm={vi.fn()} onCancel={vi.fn()} />);
    expect(screen.queryByRole("alertdialog")).toBeNull();
  });

  it("has no serious accessibility violations", async () => {
    open();
    expect(await axeViolations(document.body)).toEqual([]);
  });
});
