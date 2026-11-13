"use client";

import * as React from "react";
import { useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { useSession } from "@/lib/auth/session-provider";
import { loginUrl, safeNext } from "@/lib/navigation";
import { Account } from "@/components/account/account";

import { CreditTopUp } from "./credit-top-up";

/**
 * `/credits` (§24.7): the deep link. The same `CreditTopUp` as the in-place
 * dialog, over the account overview; closing it goes to `?next=` (a same-locale
 * path) or the profile. Signed-out visitors are sent to the login first.
 */
export function CreditsRoute() {
  const { locale } = useI18n();
  const router = useRouter();
  const { status } = useSession();

  React.useEffect(() => {
    if (status === "anonymous") router.replace(loginUrl(locale));
  }, [status, locale, router]);

  return (
    <>
      <Account view="overview" />
      <CreditTopUp onClose={() => router.push(safeNext(window.location.search, locale) ?? `/${locale}/profile`)} />
    </>
  );
}
