"use client";

import * as React from "react";
import { Dialog } from "radix-ui";
import { AlertCircle, X } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { errorMessage } from "@/lib/api/errors";
import { pricingApi, type PricingPack } from "@/lib/api/pricing";
import { track } from "@/lib/analytics";
import { useSession } from "@/lib/auth/session-provider";
import { defaultPack, type TopUpAdvisor } from "@/lib/billing/top-up";
import { useAttemptKey } from "@/lib/billing/use-attempt-key";
import { useAsync } from "@/lib/use-async";
import { FreeModeCreditsNotice } from "@/components/billing/free-chat-badge";
import { Button } from "@/components/ui/button";
import { DialogOverlay } from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";

import { PackPicker } from "./pack-picker";
import { PaymentStep } from "./payment-step";
import { SummaryDock } from "./summary-dock";

type Step = "choose" | "pay";

/** The age gate answers these globally (§24.6); the dialog shows nothing of its own. */
const AGE_GATE = new Set(["birth_date_required", "age_restricted"]);

function CatalogSkeleton() {
  const { text } = useI18n();
  return (
    <div className="top-up-body" aria-busy="true" data-testid="top-up-loading">
      <p role="status" className="sr-only">
        {text("Loading credit packs…")}
      </p>
      <div className="top-up-packs">
        <Skeleton className="top-up-skeleton top-up-skeleton-wide" />
        <div className="top-up-pack-grid">
          {Array.from({ length: 5 }, (_, i) => (
            <Skeleton key={i} className="top-up-skeleton" />
          ))}
        </div>
      </div>
    </div>
  );
}

/**
 * The credit top-up (Figma 1352:2362, docs/platform-extensions.md §24.7): one
 * dialog for the `/credits` route and for everywhere else (through
 * `TopUpProvider`, so a live chat is never left). Step 1 chooses a pack and
 * shows the summary dock; step 2 pays. Server state (the catalog) is fetched
 * here; the choice, the step and the notice are client UI state. Selecting
 * never charges: the charge is the explicit "Pay" of step 2.
 */
export function CreditTopUp({
  context,
  onClose,
  restoresFocus = false,
}: {
  context?: TopUpAdvisor;
  onClose: () => void;
  /** The caller gives the focus back itself on close (TopUpProvider), so the dialog must not. */
  restoresFocus?: boolean;
}) {
  const { text } = useI18n();
  const { status, user } = useSession();
  const catalog = useAsync(pricingApi.catalog, [user?.id], status === "authenticated");
  const attempt = useAttemptKey();
  const [step, setStep] = React.useState<Step>("choose");
  const [chosenId, setChosenId] = React.useState<string | null>(null);
  // The Welcome Pack was refused (offer_unavailable): never preselect it again here.
  const [skipIntro, setSkipIntro] = React.useState(false);
  const [notice, setNotice] = React.useState<string | null>(null);
  const titleRef = React.useRef<HTMLHeadingElement>(null);
  const stepRef = React.useRef(step);

  React.useEffect(() => {
    track("checkout_opened");
  }, []);

  // Keyboard and screen-reader users land on the new step's title.
  React.useEffect(() => {
    if (stepRef.current === step) return;
    stepRef.current = step;
    titleRef.current?.focus();
  }, [step]);

  const data = catalog.data;
  // Derived, not synced: a choice that left the catalog falls back to the default pack.
  const selected = data ? (data.packs.find((p) => p.id === chosenId) ?? defaultPack(data, skipIntro)) : null;

  const choose = (pack: PricingPack) => {
    setChosenId(pack.id);
    setNotice(null);
    track("checkout_pack_selected", { credits: pack.credits });
  };

  const offerUnavailable = (message: string) => {
    setNotice(message);
    setSkipIntro(true);
    setChosenId(null);
    setStep("choose");
    catalog.reload();
  };

  const ageGated = catalog.error !== null && AGE_GATE.has(String(catalog.error.code));
  const failed = catalog.error !== null && !data && !ageGated;
  const loading = !data && !failed;
  const empty = data !== null && data.packs.length === 0;
  const paying = step === "pay" && selected !== null;

  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <Dialog.Portal>
        <DialogOverlay overlay="blur" />
        <Dialog.Content
          className="top-up-dialog"
          data-testid="top-up"
          data-step={paying ? "pay" : "choose"}
          onOpenAutoFocus={(event) => {
            event.preventDefault();
            titleRef.current?.focus();
          }}
          onCloseAutoFocus={(event) => {
            if (restoresFocus) event.preventDefault();
          }}
        >
          <header className="top-up-header">
            <div className="top-up-heading">
              <Dialog.Title ref={titleRef} tabIndex={-1} className="top-up-title">
                {paying ? text("Complete your purchase") : text("Choose your credit pack")}
              </Dialog.Title>
              <Dialog.Description className="top-up-subtitle">{text("One-time purchase. No subscription or automatic top-up.")}</Dialog.Description>
            </div>
            <Dialog.Close asChild>
              <button type="button" className="top-up-close" aria-label={text("Close")} data-testid="top-up-close">
                <X aria-hidden size={20} />
              </button>
            </Dialog.Close>
          </header>

          {loading ? <CatalogSkeleton /> : null}

          {failed && catalog.error ? (
            <div className="top-up-body" data-testid="top-up-error">
              <div role="alert" className="top-up-state">
                <AlertCircle aria-hidden size={20} className="shrink-0" />
                <p>{text(errorMessage(catalog.error))}</p>
                {catalog.error.retryable ? (
                  <Button type="button" variant="cta" size="xl" onClick={catalog.reload} data-testid="top-up-retry-catalog">
                    {text("Try again")}
                  </Button>
                ) : null}
              </div>
            </div>
          ) : null}

          {empty ? (
            <div className="top-up-body" data-testid="top-up-empty">
              <p role="status" className="top-up-state">
                {text("No credit packs are available right now. Please check back soon.")}
              </p>
            </div>
          ) : null}

          {data && selected && !paying ? (
            <>
              <div className="top-up-body">
                <FreeModeCreditsNotice className="top-up-free" />
                {notice ? (
                  <p role="alert" className="top-up-notice" data-testid="top-up-notice">
                    <AlertCircle aria-hidden size={18} className="shrink-0" />
                    {notice}
                  </p>
                ) : null}
                <PackPicker packs={data.packs} range={data.rate_range} selectedId={selected.id} onSelect={choose} />
              </div>
              <SummaryDock
                pack={selected}
                context={context}
                onContinue={() => {
                  setNotice(null);
                  setStep("pay");
                }}
                onDismiss={onClose}
              />
            </>
          ) : null}

          {paying && selected ? (
            <PaymentStep
              pack={selected}
              attempt={attempt}
              onBack={() => setStep("choose")}
              onPurchased={onClose}
              onOfferUnavailable={offerUnavailable}
              statementDescriptor={data?.statement_descriptor}
            />
          ) : null}
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
