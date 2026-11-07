"use client";

/**
 * Requester side of the support desk (§16.2–16.3), three screens:
 *  - `support/requests`: the signed-in list (paginated)
 *  - `support/requests/<reference>`: a signed-in request with its thread
 *  - `support/<reference>#t=<token>`: the anonymous private link
 */
import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, Inbox, LinkIcon, Plus } from "lucide-react";

import { useI18n } from "@/i18n/i18n-provider";
import { supportApi } from "@/lib/api/endpoints";
import { ApiError, toApiError } from "@/lib/api/errors";
import type { CustomerTicket, CustomerTicketRow } from "@/lib/api/types";
import { useSession } from "@/lib/auth/session-provider";
import { isChatReport } from "@/lib/support/chat-report";
import { supportTopicLabels } from "@/lib/schemas/contact";
import { browserTokenEnv, classifySupportError, forgetSupportToken, normalizeReference, takeSupportToken } from "@/lib/support/support";
import { useAsync } from "@/lib/use-async";
import { usePagedList } from "@/lib/use-paged-list";
import { EmptyState } from "@/components/feedback/empty-state";
import { ErrorState } from "@/components/feedback/error-state";
import { PublicFrame } from "@/components/layout/public-frame";
import { Button } from "@/components/ui/button";
import { CursorPager } from "@/components/ui/pagination";
import { Skeleton, SkeletonText } from "@/components/ui/skeleton";
import { ReportBadge, StatusChip, TicketThread, useFormatTicketDate, useReportMeta, useTicketTitle } from "@/components/support/ticket-thread";

function Shell({ title, children, wide = false }: { title: string; children: React.ReactNode; wide?: boolean }) {
  const { text } = useI18n();
  return (
    <PublicFrame promo={false}>
      <section className="contact-page">
        <p className="eyebrow text-center">{text("Support portal")}</p>
        <h1 className="mt-4 text-center font-display text-[36px] md:text-[44px]">{title}</h1>
        <div className="mx-auto my-6 h-px w-24 bg-gold" />
        <div className="contact-card paper-page" style={wide ? { maxWidth: 900 } : undefined}>
          {children}
        </div>
      </section>
    </PublicFrame>
  );
}

function ThreadSkeleton() {
  const { text } = useI18n();
  return (
    <div aria-busy="true" data-testid="support-skeleton" className="grid gap-5">
      <p role="status" className="sr-only">
        {text("Loading…")}
      </p>
      <Skeleton className="h-4 w-28" />
      <Skeleton className="h-8 w-3/4" />
      <Skeleton className="h-6 w-48 rounded-full" />
      <Skeleton className="h-28 w-full" />
      <SkeletonText lines={3} />
    </div>
  );
}

/** Redirects anonymous visitors to the login page and back (`next`). */
function useRequireAccount(path: string) {
  const { locale } = useI18n();
  const router = useRouter();
  const { status } = useSession();
  React.useEffect(() => {
    if (status === "anonymous") router.replace(`/${locale}/login?next=${encodeURIComponent(path)}`);
  }, [status, locale, router, path]);
  return status === "authenticated";
}

/** Errors that have their own wording and call to action; everything else is the shared ErrorState. */
function SupportError({ error, context, onRetry }: { error: unknown; context: "link" | "account"; onRetry: () => void }) {
  const { text, locale } = useI18n();
  const { kind } = classifySupportError(error, context);
  if (kind === "link_invalid")
    return (
      <div role="alert" data-testid="support-link-invalid" className="grid justify-items-center gap-4 text-center">
        <LinkIcon aria-hidden className="size-9 text-amber" />
        <h2 className="text-2xl font-semibold">{text("This link has expired or is not valid")}</h2>
        <p className="max-w-prose font-light">
          {text("Private links work for 30 days and only for the request they were sent for. Send us a new request and we will pick it up from there.")}
        </p>
        <div className="flex flex-wrap justify-center gap-3">
          <Button asChild className="gold-button">
            <Link href={`/${locale}/contact`}>{text("Send a new request")}</Link>
          </Button>
          <Button asChild variant="outline" className="border-gold bg-cream">
            <Link href={`/${locale}/login?next=${encodeURIComponent(`/${locale}/support/requests`)}`}>{text("Sign in to see your requests")}</Link>
          </Button>
        </div>
      </div>
    );
  if (kind === "not_found")
    return (
      <div role="alert" data-testid="support-not-found" className="grid justify-items-center gap-4 text-center">
        <Inbox aria-hidden className="size-9 text-amber" />
        <h2 className="text-2xl font-semibold">{text("We could not find this request")}</h2>
        <p className="max-w-prose font-light">{text("It may belong to another account, or the reference is not right.")}</p>
        <Button asChild className="gold-button">
          <Link href={`/${locale}/support/requests`}>{text("Back to your requests")}</Link>
        </Button>
      </div>
    );
  return <ErrorState error={toApiError(error)} onRetry={onRetry} title={text("We could not load this request.")} />;
}

type Loader = { key: string; load: (signal: AbortSignal) => Promise<CustomerTicket>; reply: (body: string) => Promise<CustomerTicket> };

/** Loads one request and renders its thread; replies refresh it in place. */
function TicketScreen({ loader, context, enabled = true }: { loader: Loader; context: "link" | "account"; enabled?: boolean }) {
  const state = useAsync((signal) => loader.load(signal), [loader.key], enabled);
  const { data, error, loading, reload, setData } = state;
  if (data === null) {
    if (error && !loading) return <SupportError error={error} context={context} onRetry={reload} />;
    return <ThreadSkeleton />;
  }
  return (
    <>
      {error && !loading ? <ErrorState compact className="mb-5" error={error} onRetry={reload} /> : null}
      <TicketThread ticket={data} reply={loader.reply} onTicket={(t) => setData(t)} onRefresh={reload} refreshing={loading} showConversationLink={context === "account"} />
    </>
  );
}

/** `support/requests/<reference>`: a signed-in customer's request. */
export function MyTicketPage({ reference: raw }: { reference: string }) {
  const { text, locale } = useI18n();
  const reference = normalizeReference(raw) ?? raw;
  const authed = useRequireAccount(`/${locale}/support/requests/${reference}`);
  const loader = React.useMemo<Loader>(
    () => ({
      key: `me:${reference}`,
      load: (signal) => supportApi.myTicket(reference, signal),
      reply: (body) => supportApi.replyMine(reference, body),
    }),
    [reference],
  );
  return (
    <Shell title={text("Your request")} wide>
      <Link href={`/${locale}/support/requests`} className="mb-5 inline-flex items-center gap-1.5 text-sm underline underline-offset-4">
        <ArrowLeft size={14} aria-hidden />
        {text("All your requests")}
      </Link>
      {authed ? <TicketScreen loader={loader} context="account" /> : <ThreadSkeleton />}
    </Shell>
  );
}

/**
 * `support/<reference>#t=<token>`: the private link e-mailed to the
 * requester. The token is taken from the fragment, stripped from the address
 * bar and sent only as the `X-Support-Token` header.
 */
export function LinkTicketPage({ reference: raw }: { reference: string }) {
  const { text } = useI18n();
  const reference = normalizeReference(raw) ?? raw;
  // undefined: not read yet; null: none (missing or malformed); string: the token.
  const [token, setToken] = React.useState<string | null | undefined>(undefined);
  const taken = React.useRef(false);
  React.useEffect(() => {
    // Once per mount: Strict Mode runs effects twice and the fragment is gone by then.
    if (taken.current) return;
    taken.current = true;
    setToken(takeSupportToken(reference, browserTokenEnv()));
  }, [reference]);

  const loader = React.useMemo<Loader | null>(
    () =>
      token
        ? {
            key: `link:${reference}`,
            load: async (signal) => {
              try {
                return await supportApi.linkTicket(reference, token, signal);
              } catch (err) {
                // A refused token is useless: do not keep it for this tab.
                if (classifySupportError(err, "link").kind === "link_invalid") forgetSupportToken(reference);
                throw err;
              }
            },
            reply: (body) => supportApi.replyLink(reference, token, body),
          }
        : null,
    [reference, token],
  );

  return (
    <Shell title={text("Your request")} wide>
      {token === undefined ? (
        <ThreadSkeleton />
      ) : loader ? (
        <TicketScreen loader={loader} context="link" />
      ) : (
        <SupportError error={new ApiError({ code: "not_found", message: "missing token", status: 404 })} context="link" onRetry={() => {}} />
      )}
    </Shell>
  );
}

function RequestRow({ row }: { row: CustomerTicketRow }) {
  const { text, locale } = useI18n();
  const format = useFormatTicketDate();
  const titleOf = useTicketTitle();
  const meta = useReportMeta()(row);
  const report = isChatReport(row);
  return (
    <li data-testid="request-row" data-kind={report ? "chat_report" : "contact"}>
      <Link
        href={`/${locale}/support/requests/${row.reference}`}
        className="grid gap-2 rounded-xl border border-gold/50 bg-white p-4 transition-colors hover:border-gold hover:bg-cream-light focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ink"
      >
        <span className="flex flex-wrap items-center justify-between gap-2">
          <span className="font-display text-sm tracking-wider">{row.reference}</span>
          <span className="flex flex-wrap items-center gap-2">
            {report ? <ReportBadge /> : null}
            <StatusChip status={row.status} />
          </span>
        </span>
        <span className="font-semibold [overflow-wrap:anywhere]" data-testid="request-title">
          {titleOf(row)}
        </span>
        <span className="flex flex-wrap gap-x-4 text-sm text-ink/80">
          {meta ? <span className="[overflow-wrap:anywhere]">{meta}</span> : <span>{text(supportTopicLabels[row.topic] ?? supportTopicLabels.general)}</span>}
          <span>{text("Last activity {value0}", { value0: format(row.last_message_at) })}</span>
        </span>
      </Link>
    </li>
  );
}

/** `support/requests`: the signed-in customer's requests, latest activity first. */
export function SupportRequestsPage() {
  const { text, locale } = useI18n();
  const authed = useRequireAccount(`/${locale}/support/requests`);
  const list = usePagedList<CustomerTicketRow>((cursor, signal) => supportApi.myTickets(cursor, 20, signal), [], { enabled: authed });
  const first = list.items.length === 0 && !list.hasPrev;
  return (
    <Shell title={text("Your support requests")} wide>
      <div className="mb-5 flex justify-end">
        <Button asChild className="gold-button">
          <Link href={`/${locale}/contact`}>
            <Plus size={16} aria-hidden />
            {text("New request")}
          </Link>
        </Button>
      </div>
      {!authed || (list.loading && first) ? (
        <div aria-busy="true" data-testid="support-skeleton" className="grid gap-3">
          <p role="status" className="sr-only">
            {text("Loading…")}
          </p>
          {Array.from({ length: 3 }, (_, i) => (
            <Skeleton key={i} className="h-24 w-full" />
          ))}
        </div>
      ) : list.error ? (
        <ErrorState error={list.error} onRetry={list.reload} title={text("We could not load your requests.")} />
      ) : first ? (
        <EmptyState
          icon={<Inbox />}
          title={text("You have no support requests yet")}
          description={text("When you write to us, your request and our replies appear here.")}
          action={
            <Button asChild className="gold-button">
              <Link href={`/${locale}/contact`}>{text("Contact support")}</Link>
            </Button>
          }
        />
      ) : (
        <>
          <ul className="grid gap-3" aria-busy={list.loading || undefined} aria-label={text("Your support requests")}>
            {list.items.map((row) => (
              <RequestRow key={row.reference} row={row} />
            ))}
          </ul>
          <CursorPager
            page={list.page}
            hasPrev={list.hasPrev}
            hasNext={list.hasNext}
            onPrev={list.prev}
            onNext={list.next}
            loading={list.loading}
            label={text("Request pages")}
          />
        </>
      )}
    </Shell>
  );
}
