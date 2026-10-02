"use client";
import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Eye, EyeOff } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import { PublicFrame } from "@/components/layout/public-frame";
import { RadiantPanel } from "@/components/forms/radiant-panel";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";

import { minimumAdultDate } from "@/lib/calculators";

export type AuthStep =
  "email" | "password" | "birth" | "offer" | "login" | "reset";
export function AuthFlow({ step }: { step: AuthStep }) {
  const { text, locale } = useI18n();
  const router = useRouter();
  const [visible, setVisible] = useState(false);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);
  const title = {
    email: "Enter your email:",
    password: "Create password",
    birth: "Date Of Birth",
    offer: "Find love and build strong relationships with Staryield",
    login: "Enter password",
    reset: "Reset your password",
  }[step];
  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setError("");
    if (step === "password") {
      const password = String(data.get("password"));
      if (!/^(?=.*[A-Z])(?=.*\d)(?=.*[^A-Za-z0-9]).{8,}$/.test(password)) {
        setError(
          "Use 8+ characters, an uppercase letter, a number, and a special character.",
        );
        return;
      }
      if (password !== data.get("confirm")) {
        setError("The passwords do not match.");
        return;
      }
    }
    if (step === "birth") {
      const date = String(data.get("birth"));
      if (date > minimumAdultDate()) {
        setError("You must be at least 18 years old.");
        return;
      }
    }
    if (step === "login" || step === "reset") {
      setError(
        step === "login"
          ? "Sign-in is unavailable until the account service is connected."
          : "Password reset is unavailable until the account service is connected.",
      );
      return;
    }
    setPending(true);
    const next = {
      email: "/signup/password",
      password: "/signup/birth",
      birth: "/signup/offer",
      offer: "/psychics",
      login: "/profile",
      reset: "/login",
    }[step];
    router.push(`/${locale}${next}`);
  }
  return (
    <PublicFrame promo={step !== "login"}>
      <RadiantPanel className="auth-panel">
        <form
          className={`auth-card paper-page ${step === "offer" ? "offer-card" : ""}`}
          onSubmit={submit}
        >
          <div className="auth-card-body">
            <h1 className="text-center text-[32px] font-medium leading-tight">
              {text(title)}
            </h1>
            {step === "email" && (
              <>
                <label className="sr-only" htmlFor="email">
                  {text("Email ")}
                </label>
                <Input
                  id="email"
                  name="email"
                  type="email"
                  autoComplete="email"
                  placeholder={text("example@email.com")}
                  required
                  className="auth-input mt-6"
                />
                <label className="mt-5 flex items-start gap-2 text-xs font-light">
                  <input type="checkbox" required className="mt-0.5" />
                  <span>
                    {text("I have read, understand and agree to the")}{" "}
                    <Link className="underline" href={`/${locale}/terms`}>
                      {text("Terms of Service ")}
                    </Link>{" "}
                    {text("and")}{" "}
                    <Link className="underline" href={`/${locale}/privacy`}>
                      {text("Privacy Policy ")}
                    </Link>{" "}
                    {text("about the service. ")}
                  </span>
                </label>
              </>
            )}
            {(step === "password" || step === "login") && (
              <div className="mt-6 space-y-6">
                {(step === "password"
                  ? [
                      ["New password", "password"],
                      ["Confirm password", "confirm"],
                    ]
                  : [["Password", "password"]]
                ).map(([label, name]) => (
                  <label
                    className="block text-sm font-semibold tracking-wide"
                    key={name}
                  >
                    {text(label)}
                    <span className="relative mt-2 block">
                      <Input
                        className="auth-input pr-12"
                        name={name}
                        type={visible ? "text" : "password"}
                        autoComplete={
                          step === "login" ? "current-password" : "new-password"
                        }
                        placeholder={text("Enter your password")}
                        minLength={8}
                        required
                      />
                      <button
                        type="button"
                        className="absolute top-1/2 right-3 -translate-y-1/2"
                        onClick={() => setVisible(!visible)}
                        aria-label={text(
                          visible ? "Hide password" : "Show password",
                        )}
                      >
                        {visible ? <EyeOff size={20} /> : <Eye size={20} />}
                      </button>
                    </span>
                    {step === "password" && name === "password" && (
                      <span className="mt-2 block text-center text-xs font-extralight tracking-normal">
                        {text(
                          "8+ characters, 1 uppercase, 1 number, 1 special character ",
                        )}
                      </span>
                    )}
                  </label>
                ))}
                {step === "login" && (
                  <Link
                    className="block text-right text-xs underline"
                    href={`/${locale}/forgot-password`}
                  >
                    {text("Forgot password? ")}
                  </Link>
                )}
              </div>
            )}
            {step === "birth" && (
              <>
                <p className="mt-4 text-center text-sm font-light">
                  {text(
                    "We use this to generate your horoscope and detailed astrological compass. ",
                  )}
                </p>
                <label className="mt-8 block">
                  <span className="sr-only">{text("Date of birth")}</span>
                  <Input
                    className="auth-input h-14"
                    name="birth"
                    type="date"
                    min="1900-01-01"
                    max={minimumAdultDate()}
                    required
                  />
                </label>
              </>
            )}
            {step === "offer" && (
              <p className="mt-2 text-center font-light">
                {text("First 3 minutes are on us! ")}
              </p>
            )}
            {step === "reset" && (
              <label className="field mt-6">
                {text("Email ")}
                <Input
                  type="email"
                  name="email"
                  autoComplete="email"
                  required
                />
              </label>
            )}
            <p className="mt-4 text-sm text-red-800" role="alert">
              {text(error)}
            </p>
          </div>
          <Button
            type="submit"
            disabled={pending}
            className="auth-continue gold-button w-full"
          >
            {text(pending ? "Continuing…" : "Continue")}
          </Button>
          {step !== "offer" && (
            <div className="auth-legal">
              <Link href={`/${locale}/terms`}>{text("Terms of Use")}</Link> ·{" "}
              <Link href={`/${locale}/contact`}>{text("Contact Us")}</Link> ·{" "}
              <Link href={`/${locale}/privacy`}>{text("Privacy Policy")}</Link>
            </div>
          )}
        </form>
        {step === "email" && (
          <div className="mt-5 w-full max-w-[354px] text-center">
            <p className="mb-4 text-sm text-cream-light">{text("or")}</p>
            <Button
              variant="outline"
              className="w-full border-ink bg-cream"
              onClick={() =>
                setError(
                  "Google sign-up is unavailable until the account service is connected.",
                )
              }
            >
              {text("Sign up with Google ")}
            </Button>
          </div>
        )}
      </RadiantPanel>
    </PublicFrame>
  );
}
