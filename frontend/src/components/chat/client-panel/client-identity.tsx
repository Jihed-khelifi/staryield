"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { countryFlag, formatDate, regionName } from "@/lib/format";

export type ClientIdentityProps = {
  name: string;
  online: boolean;
  /** Completed sessions together (undefined while loading). */
  sessions?: number;
  /** City/region/country, only when the client agreed to share it. */
  location?: string;
  /** YYYY-MM-DD, only when the client agreed to share it. */
  birthDate?: string;
  /** Fallbacks when location / birth date are not shared. */
  countryCode?: string;
  age?: number;
};

/**
 * Name, place and birth date of the client (Figma 1172:1191), then their
 * presence and how many sessions you have had together. Location and birth
 * date only appear when the client shares them; otherwise the country and
 * age from the profile summary are shown.
 */
export function ClientIdentity({ name, online, sessions, location, birthDate, countryCode, age }: ClientIdentityProps) {
  const { text, locale } = useI18n();
  const flag = countryFlag(countryCode);
  const country = regionName(countryCode, locale);
  const place = location?.trim() || country;
  const born = birthDate ? formatDate(birthDate, locale) : "";
  return (
    <header className="client-identity">
      <h2 className="client-identity-name">{name}</h2>
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
