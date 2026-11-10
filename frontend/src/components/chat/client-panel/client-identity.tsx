"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { countryFlag, formatDate, regionName } from "@/lib/format";

export type ClientIdentityProps = {
  /** What the advisor calls this client: their private nickname when set (§23.4), else the real name. */
  name: string;
  /** The name on the account, shown underneath when a nickname replaces it. */
  accountName?: string;
  online: boolean;
  /** Completed sessions together (undefined while loading). */
  sessions?: number;
  /** City/region/country: the birth city only when the client agreed to share it. */
  location?: string;
  /** YYYY-MM-DD: advisors always see the date of birth (§22.6). */
  birthDate?: string;
  /** Fallbacks when the birth city / date are unknown. */
  countryCode?: string;
  age?: number;
};

/**
 * Name, place and birth date of the client (Figma 1172:1191), then their
 * presence and how many sessions you have had together. The birth city only
 * appears when the client shares it (the country is shown otherwise); the
 * date of birth always does, with the age ("{date} · {age} years old").
 */
export function ClientIdentity({ name, accountName, online, sessions, location, birthDate, countryCode, age }: ClientIdentityProps) {
  const { text, locale } = useI18n();
  const flag = countryFlag(countryCode);
  const country = regionName(countryCode, locale);
  const place = location?.trim() || country;
  const born = birthDate ? formatDate(birthDate, locale) : "";
  return (
    <header className="client-identity">
      <h2 className="client-identity-name">{name}</h2>
      {accountName && accountName !== name ? (
        <p className="client-identity-account" data-testid="client-account-name">
          {text("Name on the account: {value0}", { value0: accountName })}
        </p>
      ) : null}
      {place ? (
        <p className="client-identity-place" data-testid="client-location">
          {!location && flag ? (
            <span aria-hidden className="mr-1">
              {flag}
            </span>
          ) : null}
          {place}
        </p>
      ) : null}
      {born ? (
        <p className="client-identity-meta" data-testid="client-birth-date">
          <span className="sr-only">{text("Born")} </span>
          <time dateTime={birthDate}>{born}</time>
          {typeof age === "number" ? <> · {text("{count} years old", { count: age })}</> : null}
        </p>
      ) : typeof age === "number" ? (
        <p className="client-identity-meta">{text("{count} years old", { count: age })}</p>
      ) : null}
      <p className="client-identity-meta mt-2 flex flex-wrap items-center gap-x-2 gap-y-1">
        <span className="inline-flex items-center gap-1.5" data-testid="client-presence">
          <span className="presence-dot" data-online={online || undefined} aria-hidden />
          {text(online ? "Online" : "Offline")}
        </span>
        {typeof sessions === "number" ? (
          <>
            <span aria-hidden>·</span>
            <span data-testid="client-sessions">
              {sessions === 0
                ? text("First session together")
                : sessions === 1
                  ? text("1 session together")
                  : text("{count} sessions together", { count: sessions.toLocaleString(locale) })}
            </span>
          </>
        ) : null}
      </p>
    </header>
  );
}
