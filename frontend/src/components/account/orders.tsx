"use client";
import { useState } from "react";
import Link from "next/link";
import { useI18n } from "@/i18n/i18n-provider";
import { Button } from "@/components/ui/button";
import { psychicsApi, requestsApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError } from "@/lib/api/errors";
import type { Availability, CustomRequest } from "@/lib/api/types";
import { useRealtimeEvent, useSession } from "@/lib/auth/session-provider";
import { useAsync } from "@/lib/use-async";
import { useToast } from "@/components/feedback/toast-provider";

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
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={busy === r.id}
                    onClick={() => {
                      const reason = window.prompt(text("Tell us what went wrong with this delivery."));
                      if (reason && reason.trim()) void act(r.id, () => requestsApi.dispute(r.id, reason), "Thank you. Our team will review this request.");
                    }}
                  >
                    {text("Report a problem")}
                  </Button>
                ) : null}
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

/** Psychic dashboard: availability, earnings, and order fulfilment. */
export function PsychicTools() {
  const { text, locale } = useI18n();
  const toast = useToast();
  const { user, setUser } = useSession();
  const earnings = useAsync(() => psychicsApi.earnings(), []);
  const { data, error, loading, reload, replace } = useRequests();
  const [availability, setAvailability] = useState<Availability>(user?.psychic?.availability ?? "offline");
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState("");

  useRealtimeEvent("session.settled", () => earnings.reload());

  async function changeAvailability(next: Availability) {
    const previous = availability;
    setAvailability(next);
    try {
      await psychicsApi.setAvailability(next);
      if (user?.psychic) setUser({ ...user, psychic: { ...user.psychic, availability: next } });
    } catch (err) {
      setAvailability(previous);
      toast.showError(err);
    }
  }

  async function act(id: string, fn: () => Promise<CustomRequest>) {
    setBusy(id);
    try {
      replace(await fn());
    } catch (err) {
      toast.show({ tone: "error", message: text(errorMessage(toApiError(err))) });
    } finally {
      setBusy("");
    }
  }

  const s = earnings.data?.summary;
  return (
    <>
      <div className="overview-top">
        <div>
          <h1 className="text-[28px] font-semibold">{text("Advisor dashboard")}</h1>
          <p className="text-sm font-light">{text("Manage your availability, live chats and requests.")}</p>
        </div>
        <div className="overview-stats">
          <div className="stat-tile">
            <p className="text-[10px] uppercase">{text("Net earnings")}</p>
            <p className="mt-2 text-2xl">{s ? s.net_credits.toLocaleString(locale) : "—"}</p>
            <span className="text-xs">{text("credits")}</span>
          </div>
          <div className="stat-tile">
            <p className="text-[10px] uppercase">{text("Sessions")}</p>
            <p className="mt-2 text-2xl">{s ? s.sessions : "—"}</p>
          </div>
          <div className="stat-tile">
            <p className="text-[10px] uppercase">{text("Requests delivered")}</p>
            <p className="mt-2 text-2xl">{s ? s.custom_requests : "—"}</p>
          </div>
        </div>
      </div>
      <fieldset className="preference-card mb-8">
        <legend className="mb-3 text-xl font-semibold">{text("Availability")}</legend>
        <div className="flex flex-wrap gap-3">
          {(["online", "away", "offline"] as const).map((a) => (
            <label key={a} className={`status-pill cursor-pointer ${availability === a ? "online" : ""}`}>
              <input type="radio" name="availability" className="sr-only" checked={availability === a} onChange={() => void changeAvailability(a)} />
              {text(a === "online" ? "● Online" : a === "away" ? "● Away" : "● Offline")}
            </label>
          ))}
        </div>
        <p className="mt-3 text-xs font-light">
          {text("Clients can request a live chat while you are online and connected.")}{" "}
          <Link className="underline" href={`/${locale}/chatroom`}>
            {text("Open the chatroom")}
          </Link>
        </p>
      </fieldset>
      <h2 className="mb-4 text-xl font-semibold">{text("Requests")}</h2>
      {error ? (
        <p role="alert" className="text-sm">
          {text(errorMessage(error))}{" "}
          <button type="button" className="underline" onClick={reload}>
            {text("Try again")}
          </button>
        </p>
      ) : loading ? (
        <p className="text-sm" role="status">{text("Loading…")}</p>
      ) : (data?.items.length ?? 0) === 0 ? (
        <p className="text-sm font-light">{text("No requests yet.")}</p>
      ) : (
        <ul className="space-y-3">
          {data!.items.map((r) => (
            <li key={r.id} className="event-card flex-col items-start gap-2">
              <div className="flex w-full justify-between gap-3">
                <strong>
                  {text(r.service_name)} · {r.client.name}
                </strong>
                <span className="text-xs">{text(statusLabel[r.status] ?? r.status)}</span>
              </div>
              {r.client_note ? <p className="whitespace-pre-line text-sm font-light">{r.client_note}</p> : null}
              <p className="text-xs">
                {text("{value0} Credits", { value0: r.price_credits.toLocaleString(locale) })}
                {r.preferred_at ? ` · ${new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(new Date(r.preferred_at))}` : ""}
              </p>
              {r.status === "pending" ? (
                <div className="flex gap-3">
                  <Button size="sm" className="gold-button" disabled={busy === r.id} onClick={() => act(r.id, () => requestsApi.accept(r.id))}>
                    {text("Accept")}
                  </Button>
                  <Button size="sm" variant="outline" disabled={busy === r.id} onClick={() => act(r.id, () => requestsApi.decline(r.id))}>
                    {text("Decline")}
                  </Button>
                </div>
              ) : null}
              {r.status === "accepted" ? (
                <form
                  className="w-full space-y-2"
                  onSubmit={(e) => {
                    e.preventDefault();
                    const delivery = (drafts[r.id] ?? "").trim();
                    if (delivery) void act(r.id, () => requestsApi.deliver(r.id, delivery));
                  }}
                >
                  <label className="field">
                    {text("Your reading")}
                    <textarea rows={4} maxLength={10000} value={drafts[r.id] ?? ""} onChange={(e) => setDrafts((d) => ({ ...d, [r.id]: e.target.value }))} required />
                  </label>
                  <Button size="sm" type="submit" className="gold-button" disabled={busy === r.id}>
                    {text("Deliver")}
                  </Button>
                </form>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
