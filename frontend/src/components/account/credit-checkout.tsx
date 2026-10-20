"use client";
import { useEffect, useRef, useState } from "react";
import { Dialog } from "radix-ui";
import { useRouter } from "next/navigation";
import { LockKeyhole, X } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { walletApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError } from "@/lib/api/errors";
import { newId } from "@/lib/api/client";
import type { CreditPack } from "@/lib/api/types";
import { track } from "@/lib/analytics";
import { useSession } from "@/lib/auth/session-provider";
import { loginUrl, safeNext } from "@/lib/navigation";
import { useToast } from "@/components/feedback/toast-provider";

function price(cents: number, currency: string, locale: string) {
  return new Intl.NumberFormat(locale, { style: "currency", currency }).format(cents / 100);
}

export function CreditCheckout() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const toast = useToast();
  const { status, user, setWallet } = useSession();
  const [packs, setPacks] = useState<CreditPack[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [selected, setSelected] = useState<string>("");
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const [receipt, setReceipt] = useState(false);
  const [reload, setReload] = useState(0);
  // One idempotency key per purchase attempt: double clicks and retries
  // can never charge twice.
  const attemptKey = useRef(newId());

  useEffect(() => {
    if (status === "anonymous") router.replace(loginUrl(locale));
  }, [status, locale, router]);

  useEffect(() => {
    const ctrl = new AbortController();
    if (reload === 0) track("checkout_opened");
    walletApi
      .packs(ctrl.signal)
      .then((list) => {
        setLoadError("");
        setPacks(list);
        setSelected((list.find((p) => p.featured) ?? list[0])?.id ?? "");
      })
      .catch((err) => {
        if (toApiError(err).code !== "aborted") setLoadError(text(errorMessage(err)));
      });
    return () => ctrl.abort();
  }, [text, reload]);

  const close = () => router.push(safeNext(window.location.search, locale) ?? `/${locale}/profile`);

  async function pay(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!selected) return;
    setError("");
    setPending(true);
    const data = new FormData(event.currentTarget);
    try {
      const purchase = await walletApi.purchase(selected, attemptKey.current, receipt ? String(data.get("email") ?? "") : undefined);
      if (purchase.status === "succeeded") {
        if (purchase.wallet) setWallet(purchase.wallet);
        toast.show({ tone: "success", message: text("{value0} credits added to your balance.", { value0: purchase.credits.toLocaleString(locale) }) });
        close();
      } else {
        toast.show({ tone: "info", message: text("Your payment is processing. Credits appear as soon as it is confirmed.") });
        close();
      }
    } catch (err) {
      const e = toApiError(err);
      // A declined payment is final for this key; the next try is a new purchase.
      if (e.code === "payment_failed" || e.code === "validation_failed") attemptKey.current = newId();
      setError(text(errorMessage(e)));
    } finally {
      setPending(false);
    }
  }

  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open) close();
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="checkout-overlay" />
        <Dialog.Content className="checkout-card">
          <form onSubmit={pay}>
            <div className="checkout-body">
              <Dialog.Title className="font-display text-2xl">
                {text("Top up credits ")}
              </Dialog.Title>
              <Dialog.Description className="mt-2 text-sm text-gold">
                {text("Secure payment checkout via StarYield ")}
              </Dialog.Description>
              <Dialog.Close asChild>
                <button
                  type="button"
                  aria-label={text("Close checkout")}
                  className="absolute top-8 right-6 rounded-full bg-warm-gray/30 p-2"
                >
                  <X size={18} />
                </button>
              </Dialog.Close>
              {packs === null && !loadError ? (
                <p className="mt-6 text-sm" role="status">{text("Loading credit packs…")}</p>
              ) : null}
              {loadError ? (
                <div className="mt-6 text-sm" role="alert">
                  <p className="text-red-800">{loadError}</p>
                  <button type="button" className="mt-2 underline" onClick={() => {
                      setLoadError("");
                      setReload((n) => n + 1);
                    }}>
                    {text("Try again")}
                  </button>
                </div>
              ) : null}
              {packs && packs.length > 0 ? (
                <fieldset className="bundle-grid">
                  <legend className="sr-only">{text("Credit bundle")}</legend>
                  {packs.map((pack) => (
                    <label className={selected === pack.id ? "selected" : ""} key={pack.id}>
                      <input
                        className="sr-only"
                        type="radio"
                        name="bundle"
                        value={pack.id}
                        checked={selected === pack.id}
                        onChange={() => {
                          setSelected(pack.id);
                          attemptKey.current = newId();
                          track("checkout_pack_selected", { credits: pack.credits });
                        }}
                      />
                      <span className="text-xl">{pack.credits.toLocaleString(locale)}</span>
                      <span className="text-xs">{price(pack.price_cents, pack.currency, locale)}</span>
                    </label>
                  ))}
                </fieldset>
              ) : null}
              <p className="mt-6 flex items-start gap-2 rounded-lg bg-warm-gray/30 p-3 text-xs font-light">
                <LockKeyhole size={14} className="mt-0.5 shrink-0" aria-hidden />
                {text("Card details are entered with our payment provider and never reach or stay on Staryield servers.")}
              </p>
              <label className="mt-6 flex items-start gap-3 text-sm font-light">
                <input type="checkbox" required className="mt-1" />
                <span>
                  {text(
                    "I consent to this payment and authorize STARYIELD to process the selected credit bundle. ",
                  )}
                </span>
              </label>
              <label className="mt-4 flex items-start gap-3 text-sm font-light">
                <input type="checkbox" checked={receipt} onChange={(e) => setReceipt(e.target.checked)} />
                <span>
                  {text(
                    "Send automated transaction confirmation and receipt notifications to this email address. ",
                  )}
                </span>
              </label>
              {receipt ? (
                <label className="field mt-6 text-xs font-semibold">
                  {text("Notification email ")}
                  <Input name="email" type="email" autoComplete="email" defaultValue={user?.email ?? ""} required maxLength={254} />
                </label>
              ) : null}
              <p role="alert" className="mt-4 text-sm text-red-800">
                {error}
              </p>
            </div>
            <Button className="gold-button checkout-pay" type="submit" disabled={pending || !selected} aria-busy={pending}>
              {text(pending ? "Processing payment…" : "Pay & secure credits ")}
            </Button>
            <p className="flex items-center justify-center gap-2 py-6 text-xs text-sage">
              <LockKeyhole size={14} />
              {text("Encrypted 256-bit SSL transaction ")}
            </p>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
