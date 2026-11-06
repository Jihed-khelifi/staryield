"use client";
import { useId, useState } from "react";
import Link from "next/link";
import { ScrollText } from "lucide-react";
import { useI18n } from "@/i18n/i18n-provider";
import { Button } from "@/components/ui/button";
import { requestsApi } from "@/lib/api/endpoints";
import { errorMessage } from "@/lib/api/errors";
import type { CustomRequest } from "@/lib/api/types";
import { useRealtimeEvent } from "@/lib/auth/session-provider";
import { useAsync } from "@/lib/use-async";
import { usePagedList } from "@/lib/use-paged-list";
import { useToast } from "@/components/feedback/toast-provider";
import { EmptyState } from "@/components/feedback/empty-state";
import { ErrorState } from "@/components/feedback/error-state";
import { CursorPager } from "@/components/ui/pagination";
import { SkeletonList } from "@/components/ui/skeleton";
import { DeclineDialog, DeliverDialog, DisputeDialog } from "@/components/account/request-dialogs";

const statusLabel: Record<string, string> = {
  pending: "Waiting for the advisor",
  accepted: "In progress",
  delivered: "Delivered",
  declined: "Declined — refunded",
  cancelled: "Cancelled — refunded",
  expired: "Expired — refunded",
  disputed: "Under review",
  refunded: "Refunded",
};

/** Requests per page in the advisor overview. */
export const REQUESTS_PAGE_SIZE = 5;

/** The request a dialog is open for, and which action. */
type Pending = { kind: "deliver" | "decline" | "dispute"; request: CustomRequest } | null;

function useRequests() {
  const list = useAsync(() => requestsApi.list(), []);
  useRealtimeEvent<{ request: CustomRequest }>("custom_request.updated", ({ request }) =>
    list.setData(list.data ? { ...list.data, items: list.data.items.map((r) => (r.id === request.id ? request : r)) } : list.data),
  );
  useRealtimeEvent<{ request: CustomRequest }>("custom_request.created", ({ request }) =>
    list.setData(list.data ? { ...list.data, items: [request, ...list.data.items.filter((r) => r.id !== request.id)] } : list.data),
  );
  const replace = (r: CustomRequest) =>
    list.setData(list.data ? { ...list.data, items: list.data.items.map((x) => (x.id === r.id ? r : x)) } : list.data);
  return { ...list, replace };
}

/** A client's custom requests: status, delivery, cancel and dispute. */
export function ClientOrders() {
  const { text, locale } = useI18n();
  const toast = useToast();
  const { data, error, loading, reload, replace } = useRequests();
  const [busy, setBusy] = useState("");
  const [dialog, setDialog] = useState<Pending>(null);

  async function act(id: string, fn: () => Promise<CustomRequest>, done: string) {
    setBusy(id);
    try {
      replace(await fn());
      toast.show({ tone: "success", message: text(done) });
    } catch (err) {
      toast.showError(err);
    } finally {
      setBusy("");
    }
  }

  const items = data?.items ?? [];
  return (
    <section className="mt-10" aria-labelledby="orders-heading">
      <h2 id="orders-heading" className="mb-4 text-xl font-semibold">
        {text("Your requests")}
      </h2>
      {error ? (
        <p role="alert" className="text-sm">
          {text(errorMessage(error))}{" "}
          <button type="button" className="underline" onClick={reload}>
            {text("Try again")}
          </button>
        </p>
      ) : loading ? (
        <p className="text-sm" role="status">{text("Loading…")}</p>
      ) : items.length === 0 ? (
        <p className="text-sm font-light">
          {text("You have not ordered a reading yet.")}{" "}
          <Link className="underline" href={`/${locale}/psychics`}>
            {text("Browse advisors")}
          </Link>
        </p>
      ) : (
        <ul className="space-y-3">
          {items.map((r) => (
            <li key={r.id} className="event-card flex-col items-start gap-2">
              <div className="flex w-full justify-between gap-3">
                <strong>
                  {text(r.service_name)} · {r.psychic.name}
                </strong>
                <span className="text-xs">{text(statusLabel[r.status] ?? r.status)}</span>
              </div>
              <p className="text-xs font-light">
                {text("{value0} Credits", { value0: r.price_credits.toLocaleString(locale) })} ·{" "}
                {new Intl.DateTimeFormat(locale, { dateStyle: "medium" }).format(new Date(r.created_at))}
              </p>
              {r.delivery ? <p className="whitespace-pre-line text-sm">{r.delivery}</p> : null}
              <div className="flex gap-3">
                {r.status === "pending" ? (
                  <Button size="sm" variant="outline" disabled={busy === r.id} onClick={() => act(r.id, () => requestsApi.cancel(r.id), "Request cancelled and refunded.")}>
                    {text("Cancel request")}
                  </Button>
                ) : null}
                {r.status === "delivered" ? (
                  <Button size="sm" variant="outline" disabled={busy === r.id} onClick={() => setDialog({ kind: "dispute", request: r })}>
                    {text("Report a problem")}
                  </Button>
                ) : null}
              </div>
            </li>
          ))}
        </ul>
      )}
      {dialog?.kind === "dispute" ? (
        <DisputeDialog request={dialog.request} open onOpenChange={(open) => !open && setDialog(null)} onDone={replace} />
      ) : null}
    </section>
  );
}

function RequestRow({
  request: r,
  busy,
  onAccept,
  onOpen,
}: {
  request: CustomRequest;
  busy: boolean;
  onAccept: () => void;
  onOpen: (kind: "deliver" | "decline") => void;
}) {
  const { text, locale } = useI18n();
  return (
    <li className="overview-review" data-testid="advisor-request" data-status={r.status}>
      <div className="flex w-full flex-wrap items-start justify-between gap-x-3 gap-y-1">
        <p className="text-sm font-semibold">
          {text(r.service_name)} · {r.client.name}
        </p>
        <span className="text-xs">{text(statusLabel[r.status] ?? r.status)}</span>
      </div>
      {r.client_note ? <p className="mt-2 text-sm whitespace-pre-line">{r.client_note}</p> : null}
      <p className="mt-2 text-xs text-ink/75">
        {text("{value0} Credits", { value0: r.price_credits.toLocaleString(locale) })}
        {r.preferred_at ? ` · ${new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(r.preferred_at))}` : ""}
      </p>
      {r.status === "pending" ? (
        <div className="mt-3 flex gap-3">
          <Button size="sm" className="gold-button" disabled={busy} onClick={onAccept}>
            {text("Accept")}
          </Button>
          <Button size="sm" variant="outline" disabled={busy} onClick={() => onOpen("decline")}>
            {text("Decline")}
          </Button>
        </div>
      ) : null}
      {r.status === "accepted" ? (
        <Button size="sm" className="gold-button mt-3" disabled={busy} onClick={() => onOpen("deliver")}>
          {text("Deliver")}
        </Button>
      ) : null}
    </li>
  );
}

/**
 * The advisor's custom requests, five per page, with accept, decline and
 * deliver. New and updated requests arrive in real time.
 */
export function AdvisorRequests() {
  const { text } = useI18n();
  const toast = useToast();
  const headingId = useId();
  const list = usePagedList<CustomRequest>((cursor, signal) => requestsApi.list(undefined, cursor, REQUESTS_PAGE_SIZE, signal), []);
  const { replaceItem, prependItem, removeItem } = list;
  const [busy, setBusy] = useState("");
  const [dialog, setDialog] = useState<Pending>(null);

  const replace = (r: CustomRequest) => replaceItem((x) => x.id === r.id, r);
  useRealtimeEvent<{ request: CustomRequest }>("custom_request.updated", ({ request }) => replaceItem((x) => x.id === request.id, request));
  useRealtimeEvent<{ request: CustomRequest }>("custom_request.created", ({ request }) => {
    removeItem((x) => x.id === request.id);
    prependItem(request);
  });

  async function accept(r: CustomRequest) {
    setBusy(r.id);
    try {
      replace(await requestsApi.accept(r.id));
    } catch (err) {
      toast.showError(err);
    } finally {
      setBusy("");
    }
  }

  return (
    <section className="overview-card" aria-labelledby={headingId} data-testid="advisor-requests">
      <h2 id={headingId} className="overview-card-title">
        {text("Requests")}
      </h2>
      {list.error ? (
        <ErrorState error={list.error} onRetry={list.reload} compact />
      ) : list.loading ? (
        <div role="status" aria-label={text("Loading requests…")}>
          <SkeletonList rows={2} rowClassName="h-20 bg-cream" />
        </div>
      ) : list.items.length === 0 ? (
        <EmptyState icon={<ScrollText />} title={text("No requests yet.")} description={text("Clients can order written readings from your profile.")} />
      ) : (
        <ul className="grid gap-2">
          {list.items.map((r) => (
            <RequestRow key={r.id} request={r} busy={busy === r.id} onAccept={() => void accept(r)} onOpen={(kind) => setDialog({ kind, request: r })} />
          ))}
        </ul>
      )}
      <CursorPager
        page={list.page}
        hasPrev={list.hasPrev}
        hasNext={list.hasNext}
        onPrev={list.prev}
        onNext={list.next}
        loading={list.loading}
        label={text("Request pages")}
      />
      {dialog?.kind === "deliver" ? (
        <DeliverDialog request={dialog.request} open onOpenChange={(open) => !open && setDialog(null)} onDone={replace} />
      ) : dialog?.kind === "decline" ? (
        <DeclineDialog request={dialog.request} open onOpenChange={(open) => !open && setDialog(null)} onDone={replace} />
      ) : null}
    </section>
  );
}
