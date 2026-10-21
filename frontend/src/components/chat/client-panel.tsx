"use client";

import { useI18n } from "@/i18n/i18n-provider";
import { chatApi } from "@/lib/api/endpoints";
import { errorMessage } from "@/lib/api/errors";
import type { ChatMessage, Participant } from "@/lib/api/types";
import { countryFlag, formatDate, languageName, regionName } from "@/lib/format";
import { useAsync } from "@/lib/use-async";
import { ReaderAvatar } from "@/components/chat/reader-avatar";
import { SavedMessages } from "@/components/chat/reader-panel";

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h3 className="mb-2 border-b border-gold/30 pb-2 font-display text-[9px] tracking-widest uppercase">{title}</h3>
      {children}
    </section>
  );
}

function Fact({ label, value, testId, stacked }: { label: string; value: React.ReactNode; testId?: string; stacked?: boolean }) {
  return (
    <div className={stacked ? "client-fact client-fact-stacked" : "client-fact"} data-testid={testId}>
      <dt>{label}</dt>
      <dd>{value}</dd>
    </div>
  );
}

/**
 * What an advisor sees about the client of the open conversation (§9.3):
 * presence, country, age, signs, today's horoscope and their history
 * together. The API never includes balance, e-mail or birth details.
 */
export function ClientPanel({
  conversationId,
  counterpart,
  online,
  saved,
  onRemove,
}: {
  conversationId: string | null;
  counterpart: Participant;
  /** Live presence from realtime (overrides the fetched value). */
  online?: boolean;
  saved: ChatMessage[];
  onRemove: (id: string) => void;
}) {
  const { text, locale } = useI18n();
  const profile = useAsync((signal) => chatApi.clientProfile(conversationId!, signal), [conversationId], Boolean(conversationId));
  const p = profile.data && profile.data.id ? profile.data : null;
  const isOnline = online ?? p?.online ?? counterpart.status === "online";
  const name = p?.name || counterpart.name;
  const flag = countryFlag(p?.country_code);
  const country = regionName(p?.country_code, locale);
  const h = p?.horoscope;

  return (
    <div className="client-panel flex flex-col gap-5 px-4 py-10" data-testid="client-panel">
      <div className="flex items-center gap-3">
        <span className="relative shrink-0">
          <ReaderAvatar initial={name.charAt(0).toUpperCase()} image={p?.avatar_url || counterpart.avatar_url} className="size-14" />
          <span className="presence-dot presence-dot-avatar" data-online={isOnline || undefined} aria-hidden />
        </span>
        <div className="min-w-0">
          <h2 className="truncate font-display text-xl">{name}</h2>
          <p className="flex items-center gap-1.5 font-ui text-xs" data-testid="client-presence">
            <span className="presence-dot" data-online={isOnline || undefined} aria-hidden />
            {text(isOnline ? "Online" : "Offline")}
          </p>
        </div>
      </div>

      {profile.error ? (
        <div className="rounded-lg bg-cream/70 p-3 text-xs" role="alert">
          <p>{text(errorMessage(profile.error))}</p>
          <button type="button" className="mt-1 underline" onClick={profile.reload}>
            {text("Try again")}
          </button>
        </div>
      ) : profile.loading && !p ? (
        <div className="space-y-2" role="status" aria-label={text("Loading client details…")}>
          {[0, 1, 2, 3].map((i) => (
            <span key={i} className="client-skeleton" />
          ))}
        </div>
      ) : p ? (
        <>
          <Section title={text("About")}>
            <dl className="client-facts">
              {country ? (
                <Fact
                  label={text("Country")}
                  testId="client-country"
                  value={
                    <>
                      {flag ? (
                        <span aria-hidden className="mr-1">
                          {flag}
                        </span>
                      ) : null}
                      {country}
                    </>
                  }
                />
              ) : null}
              {typeof p.age === "number" ? <Fact label={text("Age")} value={text("{count} years old", { count: p.age })} /> : null}
              {p.locale ? <Fact label={text("Language")} value={languageName(p.locale, locale)} /> : null}
              <Fact label={text("Member since")} value={formatDate(p.member_since, locale, "long")} />
            </dl>
          </Section>

          {p.astro ? (
            <Section title={text("Astrology")}>
              <dl className="client-facts">
                <Fact label={text("Sun sign")} value={text(p.astro.sun_sign)} />
                {p.astro.moon_sign ? <Fact label={text("Moon sign")} value={text(p.astro.moon_sign)} /> : null}
                {p.astro.rising_sign ? <Fact label={text("Rising sign")} value={text(p.astro.rising_sign)} /> : null}
                {p.astro.life_path ? <Fact label={text("Life path")} value={String(p.astro.life_path)} /> : null}
              </dl>
              {!p.astro.time_known ? <p className="mt-2 text-[11px] font-light">{text("Birth time unknown: rising sign and houses are not available.")}</p> : null}
            </Section>
          ) : null}

          {h ? (
            <Section title={text("Today's horoscope")}>
              <div className="space-y-2 text-sm">
                <p className="text-[11px] font-light">
                  {text(h.sign)} · {formatDate(h.date, locale, "long")}
                </p>
                <dl className="client-facts">
                  {h.focus ? <Fact label={text("Focus")} value={text(h.focus)} stacked /> : null}
                  {h.mood ? <Fact label={text("Mood")} value={text(h.mood)} /> : null}
                </dl>
                {h.text ? <p className="leading-relaxed font-light">{text(h.text)}</p> : null}
              </div>
            </Section>
          ) : null}

          <Section title={text("With you")}>
            <dl className="client-facts">
              <Fact label={text("Sessions")} value={p.history.sessions.toLocaleString(locale)} testId="client-sessions" />
              <Fact label={text("Minutes")} value={p.history.minutes.toLocaleString(locale)} />
              {p.history.first_session_at ? <Fact label={text("First session")} value={formatDate(p.history.first_session_at, locale)} /> : null}
              {p.history.last_session_at ? <Fact label={text("Last session")} value={formatDate(p.history.last_session_at, locale)} /> : null}
            </dl>
            {p.history.sessions === 0 ? <p className="mt-2 text-[11px] font-light">{text("This is your first session together.")}</p> : null}
          </Section>
        </>
      ) : null}

      <SavedMessages saved={saved} onRemove={onRemove} />
    </div>
  );
}
