"use client";
import * as React from "react";
import { useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { chatApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import { newId } from "@/lib/api/client";
import type { ChatMessage, ChatSession, Conversation, Participant, Psychic, SessionSummary } from "@/lib/api/types";
import { track } from "@/lib/analytics";
import { useRealtimeEvent, useRealtimeEvents, useSession } from "@/lib/auth/session-provider";
import { loginUrl } from "@/lib/navigation";
import { logger } from "@/lib/logger";
import { useToast } from "@/components/feedback/toast-provider";
import { ChatFeed } from "@/components/chat/chat-feed";
import { ChatHeader } from "@/components/chat/chat-header";
import { ConversationList } from "@/components/chat/conversation-list";
import { MessageComposer } from "@/components/chat/message-composer";
import { ReaderPanel } from "@/components/chat/reader-panel";
import { SessionPanel } from "@/components/chat/session-panel";
import {
  elapsedSeconds,
  isOpen,
  mergeMessages,
  newClientMsgId,
  secondsUntil,
  snapshot,
  type SessionSnapshot,
  type UiMessage,
} from "@/components/chat/chat-model";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet";

type SessionEvent = { session: ChatSession; summary?: SessionSummary };

const SESSION_EVENTS = [
  "session.requested",
  "session.accepted",
  "session.started",
  "session.disconnected",
  "session.resumed",
  "session.declined",
  "session.expired",
  "session.cancelled",
  "session.ended",
] as const;

export function ChatroomShell({ initialReader }: { initialReader?: string }) {
  const { text, t, locale } = useI18n();
  const router = useRouter();
  const toast = useToast();
  const { status, user, wallet, realtime, connection } = useSession();
  const role = user?.role === "psychic" ? "psychic" : "client";
  const meId = user?.id ?? "";

  const [conversations, setConversations] = React.useState<Conversation[]>([]);
  const [inboxError, setInboxError] = React.useState<ApiError | null>(null);
  const [slug, setSlug] = React.useState<string | undefined>(initialReader);
  const [activeConvId, setActiveConvId] = React.useState<string | null>(null);
  const [counterpart, setCounterpart] = React.useState<Participant | null>(null);
  const [psychic, setPsychic] = React.useState<Psychic | null>(null);
  const [snap, setSnap] = React.useState<SessionSnapshot | null>(null);
  const [summary, setSummary] = React.useState<SessionSummary | null>(null);
  const [messages, setMessages] = React.useState<UiMessage[]>([]);
  const [cursor, setCursor] = React.useState<string | undefined>();
  const [loadingMore, setLoadingMore] = React.useState(false);
  const [saved, setSaved] = React.useState<ChatMessage[]>([]);
  const [typing, setTyping] = React.useState(false);
  const [seenUpTo, setSeenUpTo] = React.useState<string | undefined>();
  const [threadError, setThreadError] = React.useState<ApiError | null>(null);
  const [actionError, setActionError] = React.useState<ApiError | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [lowBalance, setLowBalance] = React.useState<number | null>(null);
  const [otherSession, setOtherSession] = React.useState<ChatSession | null>(null);
  const [inboxOpen, setInboxOpen] = React.useState(false);
  const [detailsOpen, setDetailsOpen] = React.useState(false);
  const [, setTick] = React.useState(0);
  const requestKey = React.useRef(newId());
  const typingTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const readTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const session = snap?.session ?? null;

  // --- authentication gate --------------------------------------------------
  React.useEffect(() => {
    if (status === "anonymous") router.replace(loginUrl(locale));
  }, [status, locale, router]);

  // --- loading ---------------------------------------------------------------
  const loadInbox = React.useCallback(async () => {
    try {
      setConversations(await chatApi.conversations());
      setInboxError(null);
    } catch (err) {
      setInboxError(toApiError(err));
    }
  }, []);

  const loadMessages = React.useCallback(async (convId: string) => {
    const [page, savedItems] = await Promise.all([chatApi.messages(convId), chatApi.saved(convId)]);
    setMessages(page.items);
    setCursor(page.next_cursor);
    setSaved(savedItems);
  }, []);

  const loadClientThread = React.useCallback(
    async (advisorSlug: string) => {
      setThreadError(null);
      try {
        const thread = await chatApi.thread(advisorSlug);
        setPsychic(thread.psychic);
        setCounterpart({ id: thread.psychic.id, role: "psychic", name: thread.psychic.name, avatar_url: thread.psychic.image_url, slug: thread.psychic.slug, status: thread.psychic.status });
        setActiveConvId(thread.conversation?.id ?? null);
        setSnap(thread.open_session ? snapshot(thread.open_session) : null);
        setOtherSession(thread.client_open_session ?? null);
        if (thread.conversation) await loadMessages(thread.conversation.id);
        else {
          setMessages([]);
          setSaved([]);
          setCursor(undefined);
        }
        track("chat_opened", { slug: advisorSlug });
      } catch (err) {
        setThreadError(toApiError(err));
      }
    },
    [loadMessages],
  );

  const openConversation = React.useCallback(
    async (conv: Conversation) => {
      setInboxOpen(false);
      setSummary(null);
      setActionError(null);
      setTyping(false);
      if (role === "client") {
        if (conv.counterpart.slug) {
          setSlug(conv.counterpart.slug);
          router.replace(`/${locale}/chatroom?reader=${conv.counterpart.slug}`, { scroll: false });
        }
        return;
      }
      setActiveConvId(conv.id);
      setCounterpart(conv.counterpart);
      setSnap(conv.open_session ? snapshot(conv.open_session) : null);
      setThreadError(null);
      try {
        await loadMessages(conv.id);
      } catch (err) {
        setThreadError(toApiError(err));
      }
    },
    [loadMessages, locale, role, router],
  );

  React.useEffect(() => {
    if (status !== "authenticated") return;
    void loadInbox();
  }, [status, loadInbox]);

  // Client: open the requested advisor (or the most recent conversation).
  React.useEffect(() => {
    if (status !== "authenticated" || role !== "client") return;
    if (slug) void loadClientThread(slug);
  }, [status, role, slug, loadClientThread]);

  React.useEffect(() => {
    if (role !== "client" || slug || conversations.length === 0) return;
    const first = conversations[0].counterpart.slug;
    if (first) setSlug(first);
  }, [role, slug, conversations]);

  // Psychic: open the conversation with a live session or request first.
  React.useEffect(() => {
    if (role !== "psychic" || activeConvId || conversations.length === 0) return;
    const live = conversations.find((c) => isOpen(c.open_session)) ?? conversations[0];
    void openConversation(live);
  }, [role, activeConvId, conversations, openConversation]);

  // --- presence: join the open session while this view is mounted ----------
  const openSessionId = isOpen(session) ? session!.id : null;
  React.useEffect(() => {
    if (!realtime || !openSessionId) return;
    let cancelled = false;
    realtime
      .joinSession<ChatSession>(openSessionId)
      .then((view) => {
        if (!cancelled && view && "status" in view) setSnap(snapshot(view));
      })
      .catch((err) => {
        const e = toApiError(err);
        if (e.code !== "realtime_unavailable" && e.code !== "timeout") setActionError(e);
        logger.debug("join failed", { code: String(e.code) });
      });
    return () => {
      cancelled = true;
      void realtime.leaveSession(openSessionId);
    };
  }, [realtime, openSessionId]);

  // Timers render from server timestamps; this only forces a repaint.
  React.useEffect(() => {
    if (!openSessionId) return;
    const id = window.setInterval(() => setTick((n) => n + 1), 1000);
    return () => window.clearInterval(id);
  }, [openSessionId]);

  // --- realtime events -------------------------------------------------------
  const upsertConversationSession = (s: ChatSession) =>
    setConversations((list) => {
      const found = list.some((c) => c.id === s.conversation_id);
      if (!found) {
        void loadInbox();
        return list;
      }
      return list.map((c) => (c.id === s.conversation_id ? { ...c, open_session: isOpen(s) ? s : undefined } : c));
    });

  const onSession = (type: string, data: SessionEvent) => {
    const s = data.session;
    upsertConversationSession(s);
    if (role === "psychic" && type === "session.requested" && (!session || !isOpen(session))) {
      toast.show({ tone: "info", message: text("{value0} would like to start a live chat.", { value0: s.client.name }) });
      const conv = conversations.find((c) => c.id === s.conversation_id);
      if (conv) void openConversation({ ...conv, open_session: s });
      else {
        setActiveConvId(s.conversation_id);
        setCounterpart(s.client);
        setSnap(snapshot(s));
        void loadMessages(s.conversation_id).catch(() => undefined);
      }
      return;
    }
    const sameConversation = s.conversation_id === activeConvId || (!activeConvId && role === "client" && s.psychic.slug === slug);
    if (!sameConversation) return;
    if (!activeConvId) {
      setActiveConvId(s.conversation_id);
      void loadMessages(s.conversation_id).catch(() => undefined);
    }
    setSnap(snapshot(s));
    if (data.summary) setSummary(data.summary);
    if (type === "session.started") setLowBalance(null);
    if (!isOpen(s)) {
      setTyping(false);
      requestKey.current = newId();
    }
  };
  useRealtimeEvents<SessionEvent>(SESSION_EVENTS, onSession);

  useRealtimeEvent<{ session_id: string; minute: number; active_seconds: number; next_billing_at: string; minutes_remaining?: number }>(
    "session.billed",
    (b) => {
      setSnap((cur) =>
        cur && cur.session.id === b.session_id
          ? { session: { ...cur.session, billed_minutes: b.minute, active_seconds: b.active_seconds, next_billing_at: b.next_billing_at, server_time: new Date().toISOString() }, receivedAt: Date.now() }
          : cur,
      );
      if (typeof b.minutes_remaining === "number" && b.minutes_remaining >= 2) setLowBalance(null);
    },
  );
  useRealtimeEvent<{ session_id: string; minutes_remaining: number }>("session.low_balance", (d) => {
    if (d.session_id === session?.id) setLowBalance(d.minutes_remaining);
  });

  useRealtimeEvent<{ message: ChatMessage }>("message.created", ({ message }) => {
    if (message.conversation_id === activeConvId) {
      setMessages((m) => mergeMessages(m, [message]));
      if (message.sender_id && message.sender_id !== meId) {
        setTyping(false);
        scheduleRead(message);
      }
    }
    setConversations((list) => {
      if (!list.some((c) => c.id === message.conversation_id)) {
        void loadInbox();
        return list;
      }
      return list
        .map((c) =>
          c.id === message.conversation_id
            ? {
                ...c,
                last_message: message,
                last_message_at: message.created_at,
                unread: c.id === activeConvId || message.sender_id === meId || message.kind === "system" ? 0 : c.unread + 1,
              }
            : c,
        )
        .sort((a, b) => (b.last_message_at ?? "").localeCompare(a.last_message_at ?? ""));
    });
  });

  useRealtimeEvent<{ session_id: string; user_id: string; typing: boolean }>("typing", (d) => {
    if (d.session_id !== session?.id || d.user_id === meId) return;
    setTyping(d.typing);
    if (typingTimer.current) clearTimeout(typingTimer.current);
    if (d.typing) typingTimer.current = setTimeout(() => setTyping(false), 5000);
  });

  useRealtimeEvent<{ conversation_id: string; reader_id: string; read_at: string }>("message.read", (d) => {
    if (d.conversation_id === activeConvId && d.reader_id !== meId) setSeenUpTo(d.read_at);
  });

  useRealtimeEvent<{ slug: string; status: Psychic["status"] }>("psychic.status_changed", (d) => {
    if (role === "client" && d.slug === psychic?.slug) {
      setPsychic((p) => (p ? { ...p, status: d.status } : p));
      setCounterpart((c) => (c ? { ...c, status: d.status } : c));
    }
  });

  // After a reconnect without replay, refetch the authoritative state.
  const resync = React.useCallback(async () => {
    void loadInbox();
    if (role === "client" && slug) {
      await loadClientThread(slug);
      return;
    }
    if (activeConvId) {
      try {
        await loadMessages(activeConvId);
      } catch (err) {
        logger.warn("resync failed", err);
      }
    }
  }, [activeConvId, loadClientThread, loadInbox, loadMessages, role, slug]);
  useRealtimeEvent("resync_required", () => void resync());
  useRealtimeEvent<{ reconnected?: boolean; resumed?: boolean }>("hello", (d) => {
    if (d.reconnected && !d.resumed) void resync();
  });

  // --- read receipts ---------------------------------------------------------
  function scheduleRead(message: ChatMessage) {
    if (readTimer.current) clearTimeout(readTimer.current);
    readTimer.current = setTimeout(() => {
      if (document.visibilityState !== "visible" || !realtime) return;
      realtime
        .request("message.read", { conversation_id: message.conversation_id, message_id: message.id })
        .then(() => setConversations((list) => list.map((c) => (c.id === message.conversation_id ? { ...c, unread: 0 } : c))))
        .catch((err) => logger.debug("mark read failed", { code: String(toApiError(err).code) }));
    }, 600);
  }

  React.useEffect(() => {
    const lastFromOther = [...messages].reverse().find((m) => m.sender_id && m.sender_id !== meId && !m.pending);
    if (lastFromOther) scheduleRead(lastFromOther);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeConvId, messages.length > 0]);

  // --- actions ---------------------------------------------------------------
  async function act<T>(fn: () => Promise<T>, after?: (v: T) => void) {
    setBusy(true);
    setActionError(null);
    try {
      const v = await fn();
      after?.(v);
      return v;
    } catch (err) {
      const e = toApiError(err);
      setActionError(e);
      track("error_displayed", { code: String(e.code), area: "chat" });
      return undefined;
    } finally {
      setBusy(false);
    }
  }

  async function startChat() {
    if (!slug) return;
    setSummary(null);
    track("chat_cta_clicked", { slug, source: "chatroom" });
    const s = await act(() => chatApi.start(slug, requestKey.current));
    if (s) {
      setSnap(snapshot(s));
      if (!activeConvId) {
        setActiveConvId(s.conversation_id);
        void loadInbox();
      }
    }
  }

  function send(body: string, retryOf?: UiMessage) {
    if (!session || session.status !== "active") return;
    const cmid = retryOf?.client_msg_id ?? newClientMsgId();
    const local: UiMessage = retryOf
      ? { ...retryOf, pending: "sending" }
      : {
          id: cmid,
          client_msg_id: cmid,
          conversation_id: session.conversation_id,
          session_id: session.id,
          sender_id: meId,
          kind: "text",
          body,
          created_at: new Date().toISOString(),
          pending: "sending",
        };
    setMessages((m) => mergeMessages(m, [local]));
    const payload = { session_id: session.id, body: local.body, client_msg_id: cmid };
    const sent = realtime
      ? realtime.request<{ message: ChatMessage }>("message.send", payload)
      : chatApi.send(session.id, local.body, cmid);
    sent
      .then(({ message }) =>
        setMessages((m) => mergeMessages(m.filter((x) => !(x.pending && x.client_msg_id === cmid)), [{ ...message, pending: undefined }])),
      )
      .catch((err) => {
        const e = toApiError(err);
        setMessages((m) => m.map((x) => (x.client_msg_id === cmid && x.pending ? { ...x, pending: "failed" } : x)));
        if (e.code !== "realtime_unavailable" && e.code !== "timeout") toast.showError(e);
        logger.warn("message send failed", e, { code: String(e.code) });
      });
  }

  async function toggleBookmark(m: UiMessage) {
    if (m.pending || m.kind !== "text") return;
    const isSaved = saved.some((s) => s.id === m.id);
    try {
      if (isSaved) {
        await chatApi.unsave(m.id);
        setSaved((s) => s.filter((x) => x.id !== m.id));
      } else {
        await chatApi.save(m.id);
        setSaved((s) => [m, ...s]);
      }
    } catch (err) {
      toast.showError(err);
    }
  }

  // --- render ------------------------------------------------------------------
  if (status !== "authenticated" || !user) {
    return (
      <main className="chat-workspace grid min-h-0 flex-1 place-items-center px-8">
        <p className="text-sm" role="status">{text("Loading your conversations…")}</p>
      </main>
    );
  }

  const savedIds = new Set(saved.map((s) => s.id));
  const rate = session?.rate_per_minute ?? psychic?.rate_per_minute ?? 0;
  const deadline = session?.status === "disconnected" ? session.grace_expires_at : session?.expires_at;
  const secondsLeft = secondsUntil(deadline, snap);
  const elapsed = elapsedSeconds(snap);
  const returnPath = `/${locale}/chatroom${slug ? `?reader=${slug}` : ""}`;
  const composerDisabled = session?.status !== "active";
  const free = wallet?.free_minutes ?? 0;
  const note =
    role === "client"
      ? free > 0
        ? text("{value0} free minutes, then {value1} credits/min", { value0: free, value1: rate })
        : text("{value0} credits/min · balance {value1}", { value0: rate, value1: (wallet?.balance ?? 0).toLocaleString(locale) })
      : t("chatroom.composer.label");

  const inbox = inboxError ? (
    <div className="p-4 text-sm" role="alert">
      <p>{text(errorMessage(inboxError))}</p>
      <button type="button" className="mt-2 underline" onClick={() => void loadInbox()}>
        {text("Try again")}
      </button>
    </div>
  ) : (
    <ConversationList conversations={conversations} activeId={activeConvId} onSelect={(c) => void openConversation(c)} />
  );
  const panel = counterpart ? (
    <ReaderPanel
      counterpart={counterpart}
      psychic={role === "client" ? psychic : null}
      saved={saved}
      onRemove={(id) => {
        const m = saved.find((s) => s.id === id);
        if (m) void toggleBookmark(m as UiMessage);
      }}
    />
  ) : null;

  const connectionBanner =
    connection.state === "offline"
      ? text("You are offline. Messages will be sent when your connection returns.")
      : connection.state === "reconnecting" || connection.state === "connecting"
        ? text("Reconnecting to live chat…")
        : "";

  return (
    <main
      aria-label={text("Chatroom")}
      className="chat-workspace grid min-h-0 flex-1 gap-4 px-8 lg:grid-cols-[260px_minmax(0,1fr)] xl:grid-cols-[260px_minmax(0,1fr)_280px]"
    >
      <aside className="hidden min-h-0 overflow-hidden rounded-xl border border-border bg-[#d3cbb9] lg:block">{inbox}</aside>
      <section className="flex min-h-0 flex-col overflow-hidden rounded-xl border border-border bg-[#d3cbb9]">
        {connectionBanner ? (
          <p className="chat-connection-banner" role="status">
            {connectionBanner}
          </p>
        ) : null}
        {counterpart ? (
          <>
            <ChatHeader
              counterpart={counterpart}
              session={session}
              elapsedSeconds={elapsed}
              rate={rate || undefined}
              onOpenInbox={() => setInboxOpen(true)}
              onOpenDetails={() => setDetailsOpen(true)}
              ending={busy}
              onEnd={() =>
                session &&
                void act(() => chatApi.end(session.id), (sum) => {
                  setSummary(sum);
                  setSnap(snapshot(sum.session));
                })
              }
            />
            {otherSession && role === "client" ? (
              <p className="chat-connection-banner" role="status">
                {text("You already have a live chat with {value0}.", { value0: otherSession.psychic.name })}{" "}
                <button type="button" className="underline" onClick={() => otherSession.psychic.slug && setSlug(otherSession.psychic.slug)}>
                  {text("Go to that chat")}
                </button>
              </p>
            ) : null}
            {lowBalance !== null && session?.status === "active" ? (
              <p className="chat-connection-banner" role="alert">
                {text("About {value0} minutes of credit left.", { value0: lowBalance })}{" "}
                <a className="underline" href={`/${locale}/credits?next=${encodeURIComponent(returnPath)}`} target="_blank" rel="noopener">
                  {text("Top up credits")}
                </a>
              </p>
            ) : null}
            {threadError ? (
              <div className="p-6 text-sm" role="alert">
                <p>{text(errorMessage(threadError))}</p>
                <button type="button" className="mt-2 underline" onClick={() => void resync()}>
                  {text("Try again")}
                </button>
              </div>
            ) : (
              <ChatFeed
                messages={messages}
                meId={meId}
                authorName={counterpart.name}
                typing={typing && session?.status === "active"}
                savedIds={savedIds}
                seenUpTo={seenUpTo}
                hasMore={Boolean(cursor)}
                loadingMore={loadingMore}
                onLoadMore={async () => {
                  if (!activeConvId || !cursor) return;
                  setLoadingMore(true);
                  try {
                    const page = await chatApi.messages(activeConvId, cursor);
                    setMessages((m) => mergeMessages(m, page.items));
                    setCursor(page.next_cursor);
                  } catch (err) {
                    toast.showError(err);
                  } finally {
                    setLoadingMore(false);
                  }
                }}
                onBookmark={toggleBookmark}
                onRetry={(m) => send(m.body, m)}
              >
                <SessionPanel
                  role={role}
                  counterpart={counterpart}
                  session={session}
                  summary={summary}
                  wallet={wallet}
                  rate={rate}
                  secondsLeft={secondsLeft}
                  busy={busy}
                  error={actionError}
                  returnPath={returnPath}
                  onStart={() => void startChat()}
                  onCancel={() => session && void act(() => chatApi.cancel(session.id), (s) => setSnap(snapshot(s)))}
                  onAccept={() => session && void act(() => chatApi.accept(session.id), (s) => setSnap(snapshot(s)))}
                  onDecline={() => session && void act(() => chatApi.decline(session.id), (s) => setSnap(snapshot(s)))}
                  onDismissSummary={() => setSummary(null)}
                />
              </ChatFeed>
            )}
            <MessageComposer
              onSend={(body) => send(body)}
              onTyping={(isTyping) => session && realtime?.send("typing", { session_id: session.id, typing: isTyping })}
              disabled={composerDisabled}
              disabledReason={text(session?.status === "disconnected" ? "Paused while reconnecting…" : "Start a live chat to send messages")}
              note={note}
            />
          </>
        ) : (
          <div className="grid flex-1 place-items-center p-8 text-center text-sm">
            <p>{text(role === "psychic" ? "Chat requests from clients appear here while you are online." : "Choose an advisor to start a conversation.")}</p>
          </div>
        )}
      </section>
      <aside className="hidden min-h-0 overflow-y-auto rounded-xl border border-border bg-[#a9a185] xl:block">{panel}</aside>
      <Sheet open={inboxOpen} onOpenChange={setInboxOpen}>
        <SheetContent side="left" className="w-[280px] gap-0 bg-cream p-0">
          <SheetHeader className="pb-0">
            <SheetTitle>{t("chatroom.inbox.title")}</SheetTitle>
          </SheetHeader>
          <div className="min-h-0 flex-1">{inbox}</div>
        </SheetContent>
      </Sheet>
      <Sheet open={detailsOpen} onOpenChange={setDetailsOpen}>
        <SheetContent side="right" className="w-[300px] gap-0 overflow-y-auto bg-sage p-0">
          <SheetHeader className="pb-0">
            <SheetTitle className="sr-only">{text(counterpart?.name ?? "")}</SheetTitle>
          </SheetHeader>
          {panel}
        </SheetContent>
      </Sheet>
    </main>
  );
}
