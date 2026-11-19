// @vitest-environment jsdom
import type * as React from "react";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";

import copyEs from "@/i18n/dictionaries/copy-es.json";
import copyFr from "@/i18n/dictionaries/copy-fr.json";
import es from "@/i18n/dictionaries/es.json";
import fr from "@/i18n/dictionaries/fr.json";
import { I18nProvider } from "@/i18n/i18n-provider";
import type { Dictionary } from "@/i18n/types";
import { stubResizeObserver } from "@/test/render";

vi.mock("@/lib/api/calculator-report", () => ({ reportEmailApi: { send: vi.fn() } }));
vi.mock("@/lib/auth/session-provider", () => ({ useSession: () => ({ status: "anonymous", user: null }) }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => (
    <a href={href} {...rest}>
      {children}
    </a>
  ),
}));

import { ReportEmailDialog } from "@/components/calculators/report-email-dialog";
import { ResultDate } from "@/components/calculators/results";

beforeAll(stubResizeObserver);
afterEach(cleanup);

const LIFE = { birth: { day: "29", month: "11", year: "1990", time: "" } } as never;

describe("the report e-mail is in French and Spanish", () => {
  it.each([
    ["fr", fr, copyFr, { title: "Enregistrez votre lecture", send: "Envoyer mon rapport", no: "Non merci", email: "Email", box: /Envoyez-moi par e-mail des horoscopes/, button: "M'envoyer ce rapport par e-mail" }],
    ["es", es, copyEs, { title: "Guarda tu lectura", send: "Enviar mi informe", no: "No, gracias", email: "Email", box: /Envíame por correo horóscopos/, button: "Enviarme este informe por correo" }],
  ])("%s", (locale, dictionary, copy, want) => {
    render(
      <I18nProvider locale={locale as "fr" | "es"} dictionary={{ ...dictionary, copy } as unknown as Dictionary}>
        <ResultDate values={LIFE} onChange={() => undefined} onEmail={() => undefined} />
        <ReportEmailDialog kind="path-of-life" values={LIFE} open onOpenChange={() => undefined} prechecked={false} manual />
      </I18nProvider>,
    );
    expect(screen.getByRole("dialog", { name: want.title })).toBeTruthy();
    expect(screen.getByRole("button", { name: want.send })).toBeTruthy();
    expect(screen.getByRole("button", { name: want.no })).toBeTruthy();
    expect(screen.getByText(want.box)).toBeTruthy();
    expect(screen.getByTestId("email-report-button").textContent).toBe(want.button);
    expect(screen.getByRole("link").getAttribute("href")).toBe(`/${locale}/privacy`);
    expect(document.body.textContent).not.toContain("Send my report");
  });
});

describe("ResultDate: the 'E-mail me this report' button", () => {
  it("shows when the report can be e-mailed and opens the dialog", () => {
    const onEmail = vi.fn();
    render(
      <I18nProvider locale="en" dictionary={{ ...({} as Dictionary), copy: {} } as unknown as Dictionary}>
        <ResultDate values={LIFE} onChange={() => undefined} onEmail={onEmail} />
      </I18nProvider>,
    );
    screen.getByTestId("email-report-button").click();
    expect(onEmail).toHaveBeenCalledTimes(1);
    expect(screen.getByTestId("email-report-button").textContent).toBe("E-mail me this report");
  });

  it("is absent for the sample result and when the report is switched off (no handler)", () => {
    render(
      <I18nProvider locale="en" dictionary={{ ...({} as Dictionary), copy: {} } as unknown as Dictionary}>
        <ResultDate values={LIFE} onChange={() => undefined} sample />
      </I18nProvider>,
    );
    expect(screen.queryByTestId("email-report-button")).toBeNull();
  });
});
