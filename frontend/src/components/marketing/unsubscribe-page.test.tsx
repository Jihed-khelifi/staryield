// @vitest-environment jsdom
import type * as React from "react";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";

import { ApiError } from "@/lib/api/errors";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import copyEs from "@/i18n/dictionaries/copy-es.json";
import fr from "@/i18n/dictionaries/fr.json";
import es from "@/i18n/dictionaries/es.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { axeViolations, renderWithI18n, stubResizeObserver } from "@/test/render";

const api = vi.hoisted(() => ({ unsubscribe: vi.fn() }));
vi.mock("@/lib/api/calculator-report", () => ({ reportEmailApi: api }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));
vi.mock("@/components/forms/auth/auth-card", () => ({
  AuthShell: ({ children }: { children: React.ReactNode }) => <main>{children}</main>,
  AuthLegal: () => null,
}));

import { UnsubscribePage } from "@/components/marketing/unsubscribe-page";

const TOKEN = "AZhs8Vq0cGJ5xW2mQnT1uA.1W7bN3kqP0dRfX9yLzC4vUe8HsTjMoIaYgDwKcEBhFQ";

beforeAll(stubResizeObserver);
beforeEach(() => {
  api.unsubscribe.mockReset();
  api.unsubscribe.mockResolvedValue({ status: "unsubscribed" });
  window.history.replaceState(null, "", `/en/unsubscribe?token=${TOKEN}`);
});
afterEach(() => {
  cleanup();
  window.history.replaceState(null, "", "/en");
});

const action = () => screen.findByTestId("unsubscribe-action");
const errorOf = (code: string, status: number) => new ApiError({ code, message: code, status });

describe("UnsubscribePage", () => {
  it("strips the token from the address bar, then waits for the click (a link scanner opening it unsubscribes nobody)", async () => {
    renderWithI18n(<UnsubscribePage />);
    await waitFor(() => expect(window.location.search).toBe(""));
    expect(screen.getByRole("heading", { name: "Unsubscribe from Staryield e-mails?" })).toBeTruthy();
    expect(api.unsubscribe).not.toHaveBeenCalled();
    expect(await axeViolations()).toEqual([]);
  });

  it("confirms on click, with the token in the request and a way back home", async () => {
    const u = userEvent.setup();
    renderWithI18n(<UnsubscribePage />);
    await u.click(await action());
    await waitFor(() => expect(api.unsubscribe).toHaveBeenCalledWith(TOKEN));
    expect(await screen.findByRole("heading", { name: "You are unsubscribed" })).toBeTruthy();
    expect(screen.getByTestId("unsubscribe").getAttribute("data-phase")).toBe("done");
    expect(screen.getByTestId("unsubscribe-next").getAttribute("href")).toBe("/en");
    expect(screen.queryByTestId("unsubscribe-action")).toBeNull();
  });

  it("shows a busy button while it works, and cannot be sent twice", async () => {
    const u = userEvent.setup();
    let release: (v: unknown) => void = () => undefined;
    api.unsubscribe.mockReturnValue(new Promise((resolve) => (release = resolve)));
    renderWithI18n(<UnsubscribePage />);
    const button = await action();
    await u.click(button);
    await waitFor(() => expect((button as HTMLButtonElement).disabled).toBe(true));
    expect(button.getAttribute("aria-busy")).toBe("true");
    expect(button.textContent).toBe("Unsubscribing…");
    await u.click(button);
    expect(api.unsubscribe).toHaveBeenCalledTimes(1);
    release({ status: "unsubscribed" });
    expect(await screen.findByRole("heading", { name: "You are unsubscribed" })).toBeTruthy();
  });

  it("explains a link that is not valid, and offers support (no retry)", async () => {
    const u = userEvent.setup();
    api.unsubscribe.mockRejectedValue(errorOf("not_found", 404));
    renderWithI18n(<UnsubscribePage />);
    await u.click(await action());
    expect(await screen.findByRole("heading", { name: "This link is not valid" })).toBeTruthy();
    expect(screen.queryByTestId("unsubscribe-action")).toBeNull();
    expect(screen.getByTestId("unsubscribe-next").getAttribute("href")).toBe("/en/contact");
  });

  it("offers a retry after a network failure, and the retry works", async () => {
    const u = userEvent.setup();
    api.unsubscribe.mockRejectedValueOnce(errorOf("network_error", 0));
    renderWithI18n(<UnsubscribePage />);
    await u.click(await action());
    expect(await screen.findByRole("heading", { name: "Something went wrong" })).toBeTruthy();
    expect(screen.getByRole("status").textContent).toContain("We couldn't reach Staryield");
    await u.click(await action());
    expect(await screen.findByRole("heading", { name: "You are unsubscribed" })).toBeTruthy();
    expect(api.unsubscribe).toHaveBeenCalledTimes(2);
    expect(api.unsubscribe).toHaveBeenLastCalledWith(TOKEN);
  });

  it("retries after a server error too", async () => {
    const u = userEvent.setup();
    api.unsubscribe.mockRejectedValueOnce(errorOf("internal_error", 500));
    renderWithI18n(<UnsubscribePage />);
    await u.click(await action());
    await u.click(await action());
    expect(await screen.findByRole("heading", { name: "You are unsubscribed" })).toBeTruthy();
  });

  it("says the link is incomplete without a token (and never calls the API)", async () => {
    window.history.replaceState(null, "", "/en/unsubscribe");
    renderWithI18n(<UnsubscribePage />);
    expect(await screen.findByRole("heading", { name: "This link is incomplete" })).toBeTruthy();
    expect(screen.queryByTestId("unsubscribe-action")).toBeNull();
    expect(api.unsubscribe).not.toHaveBeenCalled();
  });

  it("refuses an oversized token", async () => {
    window.history.replaceState(null, "", `/en/unsubscribe?token=${"a".repeat(200)}`);
    renderWithI18n(<UnsubscribePage />);
    expect(await screen.findByRole("heading", { name: "This link is incomplete" })).toBeTruthy();
  });

  it.each([
    ["fr", fr, copyFr, "Se désinscrire des e-mails de Staryield ?", "Vous êtes désinscrit", "/fr"],
    ["es", es, copyEs, "¿Darte de baja de los correos de Staryield?", "Te has dado de baja", "/es"],
  ])("is translated (%s)", async (locale, dictionary, copy, title, done, home) => {
    const u = userEvent.setup();
    render(
      <I18nProvider locale={locale as "fr" | "es"} dictionary={{ ...dictionary, copy } as unknown as Dictionary}>
        <UnsubscribePage />
      </I18nProvider>,
    );
    expect(await screen.findByRole("heading", { name: title })).toBeTruthy();
    await u.click(await action());
    expect(await screen.findByRole("heading", { name: done })).toBeTruthy();
    expect(screen.getByTestId("unsubscribe-next").getAttribute("href")).toBe(home);
  });
});
