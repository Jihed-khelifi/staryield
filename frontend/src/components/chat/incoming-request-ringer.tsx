"use client";

import * as React from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { BellRing } from "lucide-react";

import { Button } from "@/components/ui/button";
import { announce } from "@/components/feedback/live-region";
import { useI18n } from "@/i18n/i18n-provider";
import { chatApi } from "@/lib/api/endpoints";
import type { ChatSession } from "@/lib/api/types";
import { useRealtimeEvents, useSession } from "@/lib/auth/session-provider";
import {
  EMPTY_REQUESTS,
  REQUEST_EVENTS,
  applyRequestEvent,
  newestRequest,
  nextExpiryMs,
  pruneExpired,
  replaceRequests,
  requestSecondsLeft,
  type PendingRequests,
} from "@/lib/chat/incoming-requests";
import { useClientNames } from "@/lib/chat/client-alias-provider";
import { closeRequestNotification, showRequestNotification } from "@/lib/chat/request-notifications";
import { preloadRingtone, setPendingRequests, unlockRingtone, useRingtoneState } from "@/lib/chat/ringtone";
import { pushPermission } from "@/lib/push";

const EVENTS = [...REQUEST_EVENTS, "hello", "resync_required"] as const;
/** A nickname lookup that takes longer than this no longer holds back the announcement (the real name is used). */
const ALIAS_WAIT_MS = 1500;

/**
 * Advisors: rings while a chat request is pending and shows a request banner
 * outside the chatroom (docs §21.1). Renders nothing for everyone else.
 * Presence and billing are untouched: this never joins or leaves a session.
 */
export function IncomingRequestRinger() {
  const { status, user } = useSession();
  if (status !== "authenticated" || user?.role !== "psychic") return null;
  return <Ringer userId={user.id} />;
}

function Ringer({ userId }: { userId: string }) {
  const { text, locale } = useI18n();
  const pathname = usePathname();
  const ringtone = useRingtoneState();
  const [requests, setRequests] = React.useState<PendingRequests>(EMPTY_REQUESTS);
  const [now, setNow] = React.useState(() => Date.now());
  const fetchSeq = React.useRef(0);
  const notified = React.useRef(new Set<string>());
  const announced = React.useRef(new Set<string>());
  const previous = React.useRef<PendingRequests>(EMPTY_REQUESTS);
  const inChatroom = pathname === `/${locale}/chatroom` || Boolean(pathname?.startsWith(`/${locale}/chatroom/`));
  // In the chatroom the request card (role="alert") and its toast already speak.
  const inChatroomRef = React.useRef(inChatroom);
  React.useEffect(() => {
    inChatroomRef.current = inChatroom;
  });

  // Advisors see their private nickname for the client (§23.4) in the banner, the announcement and the system notification.
  const clientIds = React.useMemo(() => [...requests.values()].map((snap) => snap.session.client.id), [requests]);
  const { nameFor, isKnown } = useClientNames(clientIds);
  const [gaveUp, setGaveUp] = React.useState<ReadonlySet<string>>(() => new Set());
  React.useEffect(() => {
    const timers = clientIds.filter((id) => !isKnown(id) && !gaveUp.has(id)).map((id) => window.setTimeout(() => setGaveUp((cur) => new Set(cur).add(id)), ALIAS_WAIT_MS));
    return () => timers.forEach((t) => window.clearTimeout(t));
  }, [clientIds, isKnown, gaveUp]);
  const nameOf = React.useCallback((client: { id: string; name: string }) => nameFor(client.id, client.name), [nameFor]);

  const refresh = React.useCallback(() => {
    const seq = ++fetchSeq.current;
    chatApi.sessions("requested").then(
      (page) => {
        if (seq === fetchSeq.current) setRequests(replaceRequests(page.items as ChatSession[]));
      },
      () => undefined,
    );
  }, []);

  React.useEffect(() => {
    void preloadRingtone();
    refresh();
    const onVisible = () => {
      if (document.visibilityState === "visible") refresh();
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => document.removeEventListener("visibilitychange", onVisible);
  }, [userId, refresh]);

  useRealtimeEvents<{ session?: ChatSession }>(EVENTS, (type, data) => {
    if (type === "hello" || type === "resync_required") {
      refresh();
      return;
    }
    setRequests((current) => applyRequestEvent(current, type, data?.session));
  });

  // The ringtone follows the pending set; it stops when the advisor leaves.
  React.useEffect(() => {
    setPendingRequests(requests.size);
  }, [requests]);
  React.useEffect(() => () => setPendingRequests(0), []);
  // Observable on every page, the chatroom included (where the banner is hidden).
  React.useEffect(() => {
    const root = document.documentElement;
    root.dataset.ringtone = ringtone.status;
    return () => {
      delete root.dataset.ringtone;
    };
  }, [ringtone.status]);

  // Expiry without an event, and a countdown tick while something is pending.
  React.useEffect(() => {
    const ms = nextExpiryMs(requests);
    if (ms === null) return;
    const t = window.setTimeout(() => setRequests((c) => pruneExpired(c)), ms + 50);
    return () => window.clearTimeout(t);
  }, [requests]);
  React.useEffect(() => {
    if (requests.size === 0) return;
    const t = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(t);
  }, [requests.size]);

  // Announcements and system notifications, once per request.
  React.useEffect(() => {
    const before = previous.current;
    previous.current = requests;
    for (const [id, snap] of requests) {
      // Wait for the nickname (briefly) so the announcement and the notification use it.
      if (!isKnown(snap.session.client.id) && !gaveUp.has(snap.session.client.id)) continue;
      const clientName = nameOf(snap.session.client);
      const firstSight = !announced.current.has(id);
      if (firstSight) {
        announced.current.add(id);
        if (!inChatroomRef.current)
          announce(text("{value0} would like to start a live chat.", { value0: clientName }), "assertive");
      }
      if (!firstSight || notified.current.has(id)) continue;
      if (document.visibilityState === "hidden" && pushPermission() === "granted") {
        notified.current.add(id);
        void showRequestNotification({
          sessionId: id,
          title: text("New chat request"),
          body: text("{value0} would like to start a live chat.", { value0: clientName }),
          url: `/${locale}/chatroom`,
        });
      }
    }
    for (const id of before.keys()) {
      if (!requests.has(id)) {
        notified.current.delete(id);
        void closeRequestNotification(id);
      }
    }
  }, [requests, text, locale, isKnown, gaveUp, nameOf]);

  const newest = newestRequest(requests);
  if (!newest || inChatroom) return null;

  const more = requests.size - 1;
  const secondsLeft = requestSecondsLeft(newest, now);
  return (
    <div
      className="incoming-request-banner"
      // Not a live region: the countdown changes every second. New requests
      // are announced once through announce() above.
      role="region"
      aria-label={text("Incoming chat request")}
      data-testid="incoming-request-banner"
      data-ringtone={ringtone.status}
    >
      <BellRing className="incoming-request-icon" aria-hidden="true" size={22} />
      <div className="incoming-request-body">
        <p className="incoming-request-title">{text("{value0} would like to start a live chat.", { value0: nameOf(newest.session.client) })}</p>
        <p className="incoming-request-meta">
          {text("This request expires in {value0} seconds.", { value0: secondsLeft })}
          {more > 0 ? <span className="incoming-request-more"> {text("+{value0} more", { value0: more })}</span> : null}
        </p>
        <div className="incoming-request-actions">
          <Button asChild variant="cta" size="lg">
            <Link href={`/${locale}/chatroom`}>{text("Open chat")}</Link>
          </Button>
          {ringtone.status === "blocked" ? (
            <Button type="button" variant="outline" size="lg" onClick={() => void unlockRingtone()}>
              {text("Tap to turn on the ringtone")}
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  );
}
