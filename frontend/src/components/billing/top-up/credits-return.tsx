"use client";

import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { CircleAlert, CircleCheck, Clock, LoaderCircle } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";
import { usePurchaseStream } from "@/lib/billing/use-purchase-stream";
import { loginUrl } from "@/lib/navigation";
import { declineMessage } from "@/components/account/settings/billing/billing-format";
import { PublicFrame } from "@/components/layout/public-frame";
import { Button } from "@/components/ui/button";

const PURCHASE_ID = /^[A-Za-z0-9_-]{8,64}$/;

const noSubscription = () => () => {};

/** The `purchase` query parameter ("" on the server and when absent or malformed). */
function usePurchaseParam(): string {
  return React.useSyncExternalStore(
    noSubscription,
    () => {
      const id = new URLSearchParams(window.location.search).get("purchase") ?? "";
      return PURCHASE_ID.test(id) ? id : "";
    },
    () => "",
  );
}

/**
 * `/{locale}/credits/return?purchase=<id>` (revisions 6.0, §26.5): where the
 * rare provider redirect lands. It waits on the same purchase stream as the
 * top-up dialog and tells the payer the outcome, with the way on.
 */
export function CreditsReturn() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { status, setWallet } = useSession();
  const purchaseId = usePurchaseParam();
  const stream = usePurchaseStream(status === "authenticated" && purchaseId ? purchaseId : null);

  React.useEffect(() => {
    if (status === "anonymous") router.replace(loginUrl(locale));
  }, [status, locale, router]);

  const settled = stream.status === "final" ? stream.purchase : null;
  const wallet = settled?.status === "succeeded" ? settled.wallet : undefined;
  React.useEffect(() => {
    // The stream carries the new wallet; realtime says the same, so this only makes it instant.
    if (wallet) setWallet(wallet);
  }, [wallet, setWallet]);

  const links = (
    <div className="flex flex-wrap justify-center gap-3">
      <Button asChild variant="cta" size="lg" className="px-4">
        <Link href={`/${locale}/psychics`}>{text("Choose an Advisor")}</Link>
      </Button>
      <Button asChild variant="outline" size="lg" className="border-ink/40 px-4">
        <Link href={`/${locale}/profile`}>{text("Back to my account")}</Link>
      </Button>
    </div>
  );

  let body: React.ReactNode;
  if (status === "anonymous") body = null;
  else if (!purchaseId)
    body = (
      <State icon={<CircleAlert aria-hidden className="size-9 text-amber" />} title={text("We could not find this payment")} testId="return-missing" alert>
        <p className="max-w-prose font-light">{text("The link is incomplete. Your payments are listed in your billing history.")}</p>
        <Button asChild variant="cta" size="lg" className="px-4">
          <Link href={`/${locale}/profile/settings#billing`}>{text("Open billing")}</Link>
        </Button>
      </State>
    );
  else if (stream.status === "final" && stream.purchase.status === "succeeded")
    body = (
      <State icon={<CircleCheck aria-hidden className="size-9 text-amber" />} title={text("Payment confirmed")} testId="return-success">
        <p className="max-w-prose font-light">{text("{value0} credits added to your balance.", { value0: stream.purchase.credits.toLocaleString(locale) })}</p>
        {links}
      </State>
    );
  else if (stream.status === "final" && stream.purchase.status === "failed")
    body = (
      <State icon={<CircleAlert aria-hidden className="size-9 text-amber" />} title={text("Your payment did not go through")} testId="return-failed" alert>
        <p className="max-w-prose font-light">{text(declineMessage(stream.purchase.failure_reason))}</p>
        <Button asChild variant="cta" size="lg" className="px-4">
          <Link href={`/${locale}/credits`}>{text("Try again")}</Link>
        </Button>
      </State>
    );
  else if (stream.status === "final")
    body = (
      <State icon={<CircleAlert aria-hidden className="size-9 text-amber" />} title={text("This payment was refunded")} testId="return-refunded">
        <p className="max-w-prose font-light">{text("The money went back to your card. Your invoice and credit note are in your billing history.")}</p>
        {links}
      </State>
    );
  else if (stream.status === "timeout")
    body = (
      <State icon={<Clock aria-hidden className="size-9 text-amber" />} title={text("Your payment is processing")} testId="return-processing">
        <p className="max-w-prose font-light">{text("Your payment is processing. Credits appear as soon as it is confirmed.")}</p>
        {links}
      </State>
    );
  else if (stream.status === "error")
    body = (
      <State icon={<CircleAlert aria-hidden className="size-9 text-amber" />} title={text("We could not check your payment")} testId="return-error" alert>
        <p className="max-w-prose font-light">{text("We lost the connection while confirming your payment. You will not be charged twice. Check again, or look at your billing history.")}</p>
        <div className="flex flex-wrap justify-center gap-3">
          <Button type="button" variant="cta" size="lg" className="px-4" onClick={stream.retry}>
            {text("Try again")}
          </Button>
          <Button asChild variant="outline" size="lg" className="border-ink/40 px-4">
            <Link href={`/${locale}/profile/settings#billing`}>{text("Open billing")}</Link>
          </Button>
        </div>
      </State>
    );
  else
    body = (
      <State icon={<LoaderCircle aria-hidden className="top-up-confirming-icon size-9 text-amber" />} title={text("Confirming your payment…")} testId="return-waiting">
        <p className="max-w-prose font-light">{text("This takes a few seconds. Please keep this page open.")}</p>
      </State>
    );

  return (
    <PublicFrame promo={false}>
      <section className="contact-page">
        <div className="contact-card paper-page" aria-live="polite" data-testid="credits-return">
          {body}
        </div>
      </section>
    </PublicFrame>
  );
}

function State({ icon, title, testId, alert = false, children }: { icon: React.ReactNode; title: string; testId: string; alert?: boolean; children: React.ReactNode }) {
  return (
    <div role={alert ? "alert" : "status"} data-testid={testId} className="grid justify-items-center gap-4 text-center">
      {icon}
      <h1 className="text-2xl font-semibold">{title}</h1>
      {children}
    </div>
  );
}
