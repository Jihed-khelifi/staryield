"use client";
import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Eye, EyeOff } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import { PublicFrame } from "@/components/layout/public-frame";
import { RadiantPanel } from "@/components/forms/radiant-panel";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { authApi, meApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError } from "@/lib/api/errors";
import { track } from "@/lib/analytics";
import { useSession } from "@/lib/auth/session-provider";
import { safeNext } from "@/lib/navigation";

import { minimumAdultDate } from "@/lib/calculators";

export type AuthStep =
  | "email"
  | "password"
  | "birth"
  | "offer"
  | "login"
  | "reset"
  | "newPassword";

const SIGNUP_EMAIL = "staryield.signup.email";
const PASSWORD_RULE = /^(?=.*[A-Z])(?=.*\d)(?=.*[^A-Za-z0-9]).{8,128}$/;

function readSignupEmail(): string {
  try {
    return sessionStorage.getItem(SIGNUP_EMAIL) ?? "";
  } catch {
    return "";
  }
}

export function AuthFlow({ step }: { step: AuthStep }) {
  const { text, locale } = useI18n();
  const router = useRouter();
  const { status, user, setUser } = useSession();
  const [visible, setVisible] = useState(false);
  const [error, setError] = useState("");
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);
  const [emailDefault, setEmailDefault] = useState("");

  const title = {
    email: "Enter your email:",
    password: "Create password",
    birth: "Date Of Birth",
    offer: "Find love and build strong relationships with Staryield",
    login: "Log In",
    reset: "Reset your password",
    newPassword: "Choose a new password",
  }[step];

  useEffect(() => {
    track("signup_step_viewed", { step });
    if (step === "password" && !readSignupEmail()) router.replace(`/${locale}/signup`);
    if (step === "email" || step === "login") setEmailDefault(readSignupEmail());
  }, [step, locale, router]);

  // Steps after account creation need a session.
  useEffect(() => {
    if ((step === "birth" || step === "offer") && status === "anonymous")
      router.replace(`/${locale}/login?next=${encodeURIComponent(`/${locale}/signup/${step}`)}`);
  }, [step, status, locale, router]);

  function fail(err: unknown) {
    const e = toApiError(err);
    setFieldErrors(e.fields);
    setError(text(errorMessage(e)));
    track("error_displayed", { code: String(e.code), step });
  }

  async function submit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setError("");
    setNotice("");
    setFieldErrors({});

    if (step === "email") {
      const email = String(data.get("email")).trim();
      try {
        sessionStorage.setItem(SIGNUP_EMAIL, email);
      } catch {
        // Carried in memory only; the next step re-asks if needed.
      }
      track("signup_step_completed", { step });
      router.push(`/${locale}/signup/password`);
      return;
    }
    if (step === "password" || step === "newPassword") {
      const password = String(data.get("password"));
      if (!PASSWORD_RULE.test(password)) {
        setError(text("Use 8+ characters, an uppercase letter, a number, and a special character."));
        return;
      }
      if (password !== data.get("confirm")) {
        setError(text("The passwords do not match."));
        return;
      }
    }
    if (step === "birth") {
      const date = String(data.get("birth"));
      if (date > minimumAdultDate()) {
        setError(text("You must be at least 18 years old."));
        return;
      }
    }

    setPending(true);
    try {
      switch (step) {
        case "password": {
          const res = await authApi.register({
            email: readSignupEmail(),
            password: String(data.get("password")),
            accept_terms: true,
            locale,
          });
          setUser(res.user);
          try {
            sessionStorage.removeItem(SIGNUP_EMAIL);
          } catch {
            // Nothing to clean up.
          }
          track("signup_step_completed", { step });
          router.push(`/${locale}/signup/birth`);
          return;
        }
        case "birth": {
          const me = await meApi.update({ birth_date: String(data.get("birth")) });
          setUser(me);
          track("signup_step_completed", { step });
          router.push(`/${locale}/signup/offer`);
          return;
        }
        case "offer":
          track("signup_step_completed", { step });
          router.push(`/${locale}/psychics`);
          return;
        case "login": {
          const res = await authApi.login(String(data.get("email")), String(data.get("password")));
          setUser(res.user);
          const fallback = res.user.role === "psychic" ? `/${locale}/chatroom` : `/${locale}/profile`;
          router.push(safeNext(window.location.search, locale) ?? fallback);
          return;
        }
        case "reset":
          await authApi.forgotPassword(String(data.get("email")), locale);
          setNotice(text("If an account exists for this email, a reset link is on its way."));
          return;
        case "newPassword": {
          const token = new URLSearchParams(window.location.search).get("token") ?? "";
          await authApi.resetPassword(token, String(data.get("password")));
          setNotice(text("Your password has been updated. You can now sign in."));
          return;
        }
      }
    } catch (err) {
      fail(err);
    } finally {
      setPending(false);
    }
  }

  const fieldError = (name: string) =>
    fieldErrors[name] ? (
      <span className="mt-2 block text-xs text-red-800" role="alert">
        {text(fieldErrors[name])}
      </span>
    ) : null;

  return (
    <PublicFrame promo={step !== "login"}>
      <RadiantPanel className="auth-panel">
        <form
          className={`auth-card paper-page ${step === "offer" ? "offer-card" : ""}`}
          onSubmit={submit}
          noValidate={false}
        >
          <div className="auth-card-body">
            <h1 className="text-center text-[32px] font-medium leading-tight">
              {text(title)}
            </h1>
            {(step === "email" || step === "login" || step === "reset") && (
              <>
                <label className="sr-only" htmlFor="email">
                  {text("Email ")}
                </label>
                <Input
                  id="email"
                  name="email"
                  type="email"
                  autoComplete={step === "login" ? "username" : "email"}
                  placeholder={text("example@email.com")}
                  defaultValue={emailDefault}
                  key={emailDefault}
                  maxLength={254}
                  required
                  className="auth-input mt-6"
                />
                {fieldError("email")}
              </>
            )}
            {step === "email" && (
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
            )}
            {(step === "password" || step === "login" || step === "newPassword") && (
              <div className="mt-6 space-y-6">
                {(step === "login"
                  ? [["Password", "password"]]
                  : [
                      ["New password", "password"],
                      ["Confirm password", "confirm"],
                    ]
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
                        minLength={step === "login" ? 1 : 8}
                        maxLength={128}
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
                    {step !== "login" && name === "password" && (
                      <span className="mt-2 block text-center text-xs font-extralight tracking-normal">
                        {text(
                          "8+ characters, 1 uppercase, 1 number, 1 special character ",
                        )}
                      </span>
                    )}
                    {fieldError(name)}
                  </label>
                ))}
                {step === "login" && (
                  <div className="flex justify-between text-xs">
                    <Link className="underline" href={`/${locale}/signup`}>
                      {text("Create an account")}
                    </Link>
                    <Link className="underline" href={`/${locale}/forgot-password`}>
                      {text("Forgot password? ")}
                    </Link>
                  </div>
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
                    defaultValue={user?.profile.birth_date ?? ""}
                    required
                  />
                </label>
                {fieldError("birth_date")}
              </>
            )}
            {step === "offer" && (
              <p className="mt-2 text-center font-light">
                {text("First 3 minutes are on us! ")}
              </p>
            )}
            <p className="mt-4 text-sm text-red-800" role="alert">
              {error}
            </p>
            {error && step === "password" && fieldErrors.email ? (
              <Link className="text-sm underline" href={`/${locale}/login`}>
                {text("Log In")}
              </Link>
            ) : null}
            {notice ? (
              <p className="mt-2 text-sm" role="status">
                {notice}{" "}
                {step === "newPassword" ? (
                  <Link className="underline" href={`/${locale}/login`}>
                    {text("Log In")}
                  </Link>
                ) : null}
              </p>
            ) : null}
          </div>
          <Button
            type="submit"
            disabled={pending}
            aria-busy={pending}
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
                  text("Google sign-up is not available yet. Please continue with your email."),
                )
              }
            >
              {text("Sign up with Google ")}
            </Button>
            <p className="mt-4 text-sm text-cream-light">
              <Link className="underline" href={`/${locale}/login`}>
                {text("Already have an account? Log in")}
              </Link>
            </p>
          </div>
        )}
      </RadiantPanel>
    </PublicFrame>
  );
}
