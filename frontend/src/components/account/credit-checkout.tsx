"use client";
import { useState } from "react";
import { Dialog } from "radix-ui";
import { useRouter } from "next/navigation";
import { LockKeyhole, X } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

const bundles = [
  { credits: 250, price: "$9.99" },
  { credits: 600, price: "$19.99" },
  { credits: 1500, price: "$39.99" },
];
export function CreditCheckout() {
  const { text, locale } = useI18n();
  const router = useRouter();
  const [selected, setSelected] = useState(600);
  const [error, setError] = useState("");
  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open) router.push(`/${locale}/profile`);
      }}
    >
      <Dialog.Portal>
        <Dialog.Overlay className="checkout-overlay" />
        <Dialog.Content className="checkout-card">
          <form
            onSubmit={(event) => {
              event.preventDefault();
              setError(
                "Payment is unavailable. No charge has been made; the payment service is not connected.",
              );
            }}
          >
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
              <fieldset className="bundle-grid">
                <legend className="sr-only">{text("Credit bundle")}</legend>
                {bundles.map((bundle) => (
                  <label
                    className={selected === bundle.credits ? "selected" : ""}
                    key={bundle.credits}
                  >
                    <input
                      className="sr-only"
                      type="radio"
                      name="bundle"
                      value={bundle.credits}
                      checked={selected === bundle.credits}
                      onChange={() => setSelected(bundle.credits)}
                    />
                    <span className="text-xl">
                      {text(bundle.credits.toLocaleString(locale))}
                    </span>
                    <span className="text-xs">{text(bundle.price)}</span>
                  </label>
                ))}
              </fieldset>
              <label className="field mt-6 font-display text-xs">
                {text("Cardholder name ")}
                <Input
                  name="holder"
                  placeholder={text("e.g. Cassandra Moon")}
                  autoComplete="cc-name"
                  required
                />
              </label>
              <label className="field mt-4 font-display text-xs">
                {text("Card number ")}
                <Input
                  name="number"
                  inputMode="numeric"
                  pattern="[0-9 ]{13,23}"
                  placeholder={"0000 0000 0000 0000"}
                  autoComplete="cc-number"
                  required
                  maxLength={23}
                />
              </label>
              <div className="mt-4 grid grid-cols-2 gap-3">
                <label className="field font-display text-xs">
                  {text("Expiry date ")}
                  <Input
                    name="expiry"
                    placeholder={text("MM / YY")}
                    autoComplete="cc-exp"
                    pattern="(0[1-9]|1[0-2])\s*/\s*[0-9]{2}"
                    required
                  />
                </label>
                <label className="field font-display text-xs">
                  {text("CVV ")}
                  <Input
                    name="cvv"
                    type="password"
                    inputMode="numeric"
                    autoComplete="cc-csc"
                    pattern="[0-9]{3,4}"
                    placeholder={"123"}
                    maxLength={4}
                    required
                  />
                </label>
              </div>
              <label className="mt-6 flex items-start gap-3 text-sm font-light">
                <input type="checkbox" required className="mt-1" />
                <span>
                  {text(
                    "I consent to this payment and authorize STARYIELD to process the selected credit bundle. ",
                  )}
                </span>
              </label>
              <label className="mt-4 flex items-start gap-3 text-sm font-light">
                <input type="checkbox" />
                <span>
                  {text(
                    "Send automated transaction confirmation and receipt notifications to this email address. ",
                  )}
                </span>
              </label>
              <label className="field mt-6 text-xs font-semibold">
                {text("Notification email ")}
                <Input
                  name="email"
                  type="email"
                  autoComplete="email"
                  placeholder={text("Cassandra Moon")}
                  required
                />
              </label>
              <p role="alert" className="mt-4 text-sm text-red-800">
                {text(error)}
              </p>
            </div>
            <Button className="gold-button checkout-pay" type="submit">
              {text("Pay & secure credits ")}
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
