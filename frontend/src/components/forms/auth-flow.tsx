"use client";

import { BirthInfoForm } from "@/components/forms/auth/birth-info-form";
import { ForgotPasswordForm } from "@/components/forms/auth/forgot-password-form";
import { LoginForm } from "@/components/forms/auth/login-form";
import { OfferStep } from "@/components/forms/auth/offer-step";
import { ResetPasswordForm } from "@/components/forms/auth/reset-password-form";
import { SignupEmailForm } from "@/components/forms/auth/signup-email-form";
import { SignupPasswordForm } from "@/components/forms/auth/signup-password-form";

export type AuthStep = "email" | "password" | "birth" | "offer" | "login" | "reset" | "newPassword";

/**
 * Sign-up, sign-in and password-reset screens. Each step is its own
 * react-hook-form + yup form (components/forms/auth/*) on the shared logo
 * page; this keeps the route table in [...screen]/page.tsx unchanged.
 */
export function AuthFlow({ step }: { step: AuthStep }) {
  switch (step) {
    case "email":
      return <SignupEmailForm />;
    case "password":
      return <SignupPasswordForm />;
    case "birth":
      return <BirthInfoForm />;
    case "offer":
      return <OfferStep />;
    case "login":
      return <LoginForm />;
    case "reset":
      return <ForgotPasswordForm />;
    case "newPassword":
      return <ResetPasswordForm />;
  }
}
