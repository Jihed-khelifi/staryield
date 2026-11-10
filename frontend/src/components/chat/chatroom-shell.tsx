"use client";
import * as React from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";

import { useI18n } from "@/i18n/i18n-provider";
import { chatApi } from "@/lib/api/endpoints";
import { errorMessage, toApiError, type ApiError } from "@/lib/api/errors";
import { newId } from "@/lib/api/client";
import type {
  ChatMessage,
  ChatSession,
  Conversation,
  MessageLikedEvent,
  MessageReadEvent,
  Participant,
  Psychic,
  ReplyPreview,
  Review,
  SessionBilledEvent,
  SessionSummary,
} from "@/lib/api/types";
import { firstFile, imageSize, seedAttachment } from "@/lib/chat/attachments";
import { outboxMessage } from "@/lib/chat/outbox";
import { useClientAliasActions, useClientName, useClientNames, usePrimeClientAliases } from "@/lib/chat/client-alias-provider";
import { track } from "@/lib/analytics";
import { useRealtimeEvent, useRealtimeEvents, useSession } from "@/lib/auth/session-provider";
import { loginUrl } from "@/lib/navigation";
import { shouldToastOtherConversation } from "@/lib/chat/chat-alerts";
import { useChatUnread } from "@/lib/chat/chat-unread-provider";
import { useNotifications } from "@/lib/notifications/notifications-provider";
import { useBlockStrayFileDrops, useFileDrop } from "@/lib/use-file-drop";
import { useVisualViewport } from "@/lib/use-visual-viewport";
import { useFreeMode } from "@/lib/site-config/site-config-provider";
import { logger } from "@/lib/logger";
import { usePagedList } from "@/lib/use-paged-list";
import { announce } from "@/components/feedback/live-region";
import { ErrorState } from "@/components/feedback/error-state";
import { useToast } from "@/components/feedback/toast-provider";
import { ChatFeed, type Announcement, type ChatFeedHandle } from "@/components/chat/chat-feed";
import { MessageActionsProvider, type MessageActions } from "@/components/chat/message-actions-context";
import { ChatHeader, ChatroomTopBar, useEarningsLabel } from "@/components/chat/chat-header";
import { SessionTools, SwitcherAccount, SwitcherHeader, SwitcherQuickLinks } from "@/components/chat/chat-mobile";
import type { ReportIssueTarget } from "@/components/chat/report-issue";
import { ComposerNote } from "@/components/chat/composer-note";
import { ChatPushPrompt } from "@/components/chat/push-prompt";
import { ConversationList } from "@/components/chat/conversation-list";
import { composerAvailability, reasonCopy } from "@/components/chat/composer-availability";
import { MessageComposer, type ComposerHandle, type ComposerSubmit } from "@/components/chat/message-composer";
import { useOffSessionAllowance } from "@/components/chat/use-off-session-allowance";
import { ReaderPanel } from "@/components/chat/reader-panel";
import { ClientPanel } from "@/components/chat/client-panel";
import { SAVED_PAGE_SIZE } from "@/components/chat/saved-list";
import { ReviewDialog } from "@/components/reviews/review-form";
import { SessionPanel } from "@/components/chat/session-panel";
import { useConversationTimeline } from "@/components/chat/use-conversation-timeline";
import { useInbox, withMessage } from "@/components/chat/use-inbox";
import { useJumpToMessage } from "@/components/chat/use-jump-to-message";
import { useMessageLikes, withLikeOverrides } from "@/components/chat/use-message-likes";
import { useOutbox } from "@/components/chat/use-outbox";
import {
  elapsedSeconds,
  isActionable,
  isOpen,
  mergeMessages,
  newClientMsgId,
  newestIncomingSeq,
  replyPreviewOf,
  secondsUntil,
  sessionEarnings,
  sessionRate,
  snapshot,
  type SessionSnapshot,
  type UiMessage,
} from "@/components/chat/chat-model";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
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

/** Read receipts wait this long, so a burst of messages costs one request. */
const READ_DEBOUNCE_MS = 600;
/** Screen-reader announcements kept in the live log. */
const LOG_SIZE = 5;

export function ChatroomShell({ initialReader, initialReview }: { initialReader?: string; initialReview?: string }) {
  const { text, t, locale } = useI18n();
  const router = useRouter();
  const toast = useToast();
  const { status, user, wallet, realtime, connection } = useSession();
  const role = user?.role === "psychic" ? "psychic" : "client";
  const meId = user?.id ?? "";
  const freeMode = useFreeMode();
  // The composer stays above the phone keyboard (iOS Safari does not resize the layout).
  useVisualViewport();
  const chatUnread = useChatUnread();
  const notifications = useNotifications();

  const [slug, setSlug] = React.useState<string | undefined>(initialReader);
  const [activeConvId, setActiveConvId] = React.useState<string | null>(null);
  const [counterpart, setCounterpart] = React.useState<Participant | null>(null);
  const [psychic, setPsychic] = React.useState<Psychic | null>(null);
  const [snap, setSnap] = React.useState<SessionSnapshot | null>(null);
  const [summary, setSummary] = React.useState<SessionSummary | null>(null);
  const [typing, setTyping] = React.useState(false);
  const [threadError, setThreadError] = React.useState<ApiError | null>(null);
  const [actionError, setActionError] = React.useState<ApiError | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [lowBalance, setLowBalance] = React.useState<number | null>(null);
  const [otherSession, setOtherSession] = React.useState<ChatSession | null>(null);
  const [inboxOpen, setInboxOpen] = React.useState(false);
  const [detailsOpen, setDetailsOpen] = React.useState(false);
  const [pendingSwitch, setPendingSwitch] = React.useState<Conversation | null>(null);
  // Read positions of the conversation on screen (mine and the other side's).
  const [reads, setReads] = React.useState<{ conv: string | null; mine: number; peer: number }>({ conv: null, mine: 0, peer: 0 });
  const [images, setImages] = React.useState<UiMessage[]>([]);
  const [savedOverride, setSavedOverride] = React.useState<Record<string, boolean>>({});
  const [log, setLog] = React.useState<Announcement[]>([]);
  const [scrollToken, setScrollToken] = React.useState(0);
  // Client state of replies and likes (§23.2, §23.3): the server state (`reply_to`, `liked_by`) lives on the timeline messages.
  const [replyTargets, setReplyTargets] = React.useState<Record<string, ReplyPreview>>({});
  const [atBottom, setAtBottom] = React.useState(true);
  const [visible, setVisible] = React.useState(true);
  // Deep link from the `chat.review_prompt` notification (clients only). The
  // prop can change without a remount (client navigation to the same reader).
  const [dismissedReview, setDismissedReview] = React.useState<string | null>(null);
  // Once the param is gone, the same link may open the dialog again later.
  if (!initialReview && dismissedReview !== null) setDismissedReview(null);
  const reviewOpen = Boolean(initialReview) && dismissedReview !== initialReview;
  const [, setTick] = React.useState(0);
  const requestKey = React.useRef(newId());
  const typingTimer = React.useRef<ReturnType<typeof setTimeout> | null>(null);
  const session = snap?.session ?? null;
  // The open session of the conversation on screen (messages go to it).
  const liveSession = session && isOpen(session) && (!activeConvId || session.conversation_id === activeConvId) ? session : null;
  // Messages go into the session while it is active or paused; otherwise they are written outside a live chat (§22.3).
  const inLiveChat = liveSession?.status === "active" || liveSession?.status === "disconnected";
  const requestPending = liveSession?.status === "requested" || liveSession?.status === "accepted";
  const composerRef = React.useRef<ComposerHandle>(null);
  const feedRef = React.useRef<ChatFeedHandle>(null);
  // Advisors see their private nickname for the client everywhere (§23.4); `counterpart.name` stays the name on the account.
  const aliasActions = useClientAliasActions();
  const counterpartDisplay = useClientName(role === "psychic" ? counterpart?.id : null, counterpart?.name ?? "");
  const { nameFor } = useClientNames();
  const allowance = useOffSessionAllowance(activeConvId, inLiveChat);
  const availability = composerAvailability({
    role,
    hasConversation: Boolean(activeConvId),
    live: inLiveChat,
    requestPending,
    allowance: allowance.allowance,
    allowanceLoading: allowance.loading,
    allowanceError: allowance.error,
  });
  const unavailableReason = availability.enabled ? "" : text(reasonCopy(availability.reason, role), { value0: allowance.allowance?.limit ?? 0 });
  // The whole conversation is the drop target (§22.4); a file dropped elsewhere on the page never navigates away.
  const drop = useFileDrop({
    disabled: !counterpart || !availability.enabled,
    onFiles: (files) => {
      const file = firstFile(files);
      if (file) void composerRef.current?.attach(file);
    },
    onBlocked: () => {
      if (!counterpart) return;
      toast.show({ tone: "info", message: unavailableReason });
      announce(unavailableReason, "assertive");
    },
  });
  useBlockStrayFileDrops();

  // --- authentication gate --------------------------------------------------
  React.useEffect(() => {
    if (status === "anonymous") router.replace(loginUrl(locale));
  }, [status, locale, router]);

  // --- inbox (cursor pages) ----------------------------------------------------
  const activeConvRef = React.useRef<string | null>(null);
  React.useEffect(() => {
    activeConvRef.current = activeConvId;
  });
  const inbox = useInbox(status === "authenticated", {
    onFirstPage: (items) => {
      if (role === "client") {
        setSlug((cur) => cur ?? items[0]?.counterpart.slug);
      } else if (items.length > 0 && !activeConvRef.current) {
        const live = items.find((c) => isOpen(c.open_session)) ?? items[0];
        setActiveConvId(live.id);
        setCounterpart(live.counterpart);
        setSnap(live.open_session ? snapshot(live.open_session) : null);
        setReads({ conv: live.id, mine: live.read_seq ?? 0, peer: live.peer_read_seq ?? 0 });
      }
    },
  });

  // --- history (strict seq order, gap filling) --------------------------------
  const counterpartName = counterpartDisplay;
  // Rows and the open conversation already carry the nickname: cache it before anything asks.
  const aliasEntries = React.useMemo(() => {
    if (role !== "psychic") return [];
    const entries = inbox.items.map((c) => ({ clientId: c.counterpart.id, alias: c.counterpart.alias }));
    if (counterpart) entries.push({ clientId: counterpart.id, alias: counterpart.alias });
    return entries;
  }, [role, inbox.items, counterpart]);
  usePrimeClientAliases(aliasEntries);
  const pushLog = React.useCallback((entries: Announcement[]) => {
    if (entries.length) setLog((cur) => [...cur, ...entries].slice(-LOG_SIZE));
  }, []);
  const thread = useConversationTimeline(activeConvId, {
    onBackfilled: (fresh) => {
      // Messages that arrived while disconnected: announce what came in.
      const incoming = fresh.filter((m) => m.sender_id && m.sender_id !== meId && m.kind !== "system");
      if (incoming.length === 1) pushLog([{ id: `b-${incoming[0].id}`, text: describeIncoming(incoming[0]) }]);
      else if (incoming.length > 1)
        pushLog([{ id: `b-${incoming.at(-1)!.id}`, text: text("{value0} new messages from {value1}", { value0: incoming.length, value1: counterpartName }) }]);
    },
  });
  const timeline = thread.timeline;
  const { receive, fillGaps, refreshNewest } = thread;

  function describeIncoming(m: ChatMessage): string {
    const body = m.kind === "image" ? m.body.trim() || text("Photo") : m.body;
    return text("{value0}: {value1}", { value0: counterpartName, value1: body.length > 280 ? `${body.slice(0, 280)}…` : body });
  }

  // --- outbox (text messages survive refresh and offline spells) --------------
  const outbox = useOutbox({
    userId: meId,
    conversationId: activeConvId,
    liveSessionId: liveSession?.id ?? null,
    realtime,
    connection: connection.state,
    onDelivered: (message, fresh) => {
      receive([message]);
      allowance.update(fresh);
    },
    onFailed: (_entry, error) => {
      announce(text("A message was not delivered. You can retry it."), "assertive");
      // A refusal means the allowance is not what the composer believed: read it again.
      if (String(error.code).startsWith("off_session_") || error.code === "recipient_unavailable") allowance.reload();
    },
  });
  // Entries the server already confirmed (seen in history or events) leave the outbox.
  const confirmedIds = React.useMemo(() => {
    const ids: string[] = [];
    for (const m of timeline.messages) if (m.client_msg_id && m.sender_id === meId) ids.push(m.client_msg_id);
    return ids;
  }, [timeline.messages, meId]);
  React.useEffect(() => {
    if (outbox.entries.length) outbox.confirm(confirmedIds);
  }, [confirmedIds, outbox]);

  // --- saved messages (cursor pages of five) -----------------------------------
  const saved = usePagedList<ChatMessage>(
    (cursor, signal) => chatApi.savedPage(activeConvId ?? undefined, cursor, SAVED_PAGE_SIZE, signal),
    [activeConvId ?? ""],
    { enabled: Boolean(activeConvId) },
  );

  // --- client: the conversation with the selected advisor ----------------------
  const [threadNonce, setThreadNonce] = React.useState(0);
  React.useEffect(() => {
    if (status !== "authenticated" || role !== "client" || !slug) return;
    const ctrl = new AbortController();
    chatApi.thread(slug, ctrl.signal).then(
      (res) => {
        setThreadError(null);
        setPsychic(res.psychic);
        setCounterpart({ id: res.psychic.id, role: "psychic", name: res.psychic.name, avatar_url: res.psychic.image_url, slug: res.psychic.slug, status: res.psychic.status });
        setActiveConvId(res.conversation?.id ?? null);
        setSnap(res.open_session ? snapshot(res.open_session) : null);
        setOtherSession(res.client_open_session ?? null);
        if (res.conversation) setReads((r) => mergeReads(r, res.conversation!.id, res.conversation!.read_seq, res.conversation!.peer_read_seq));
        track("chat_opened", { slug });
      },
      (err) => {
        const e = toApiError(err);
        if (!ctrl.signal.aborted && e.code !== "aborted") setThreadError(e);
      },
    );
    return () => ctrl.abort();
  }, [status, role, slug, threadNonce]);

  const openConversation = (conv: Conversation) => {
    setInboxOpen(false);
    setSummary(null);
    setActionError(null);
    setTyping(false);
    if (role === "client") {
      if (conv.counterpart.slug && conv.counterpart.slug !== slug) {
        setSlug(conv.counterpart.slug);
        router.replace(`/${locale}/chatroom?reader=${conv.counterpart.slug}`, { scroll: false });
      }
      return;
    }
    setActiveConvId(conv.id);
    setCounterpart(conv.counterpart);
    setSnap(conv.open_session ? snapshot(conv.open_session) : null);
    setReads({ conv: conv.id, mine: conv.read_seq ?? 0, peer: conv.peer_read_seq ?? 0 });
    setLog([]);
  };

  // Clients in a live chat confirm before switching advisors (Figma 1261:1537).
  const selectConversation = (conv: Conversation) => {
    if (role === "client" && liveSession && conv.counterpart.slug && conv.counterpart.slug !== slug) {
      setPendingSwitch(conv);
      return;
    }
    openConversation(conv);
  };
  const confirmSwitch = async () => {
    const target = pendingSwitch;
    setPendingSwitch(null);
    if (!target || !liveSession) return;
    const ended = await act(() => (liveSession.status === "requested" ? chatApi.cancel(liveSession.id) : chatApi.end(liveSession.id).then((sum) => sum.session)));
    if (ended) openConversation(target);
  };

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

  // Read receipts only go out while the page is visible.
  React.useEffect(() => {
    const onVisibility = () => setVisible(document.visibilityState === "visible");
    document.addEventListener("visibilitychange", onVisibility);
    return () => document.removeEventListener("visibilitychange", onVisibility);
  }, []);

  // --- realtime events -------------------------------------------------------
  const upsertConversationSession = (s: ChatSession) => {
    if (!inbox.items.some((c) => c.id === s.conversation_id)) {
      inbox.refresh();
      return;
    }
    inbox.patch((list) => list.map((c) => (c.id === s.conversation_id ? { ...c, open_session: isOpen(s) ? s : undefined } : c)));
  };

  const onSession = (type: string, data: SessionEvent) => {
    const s = data.session;
    upsertConversationSession(s);
    if (role === "psychic" && type === "session.requested" && (!session || !isOpen(session))) {
      // The client's nickname (§23.4) when the advisor has one; the toast waits for it briefly.
      void aliasActions.resolve(s.client.id).then((alias) =>
        toast.show({ tone: "info", message: text("{value0} would like to start a live chat.", { value0: alias ?? s.client.name }) }),
      );
      const conv = inbox.items.find((c) => c.id === s.conversation_id);
      // The client just sent a request, so they are connected (a stale inbox row may still say offline).
      const requester = { ...(conv?.counterpart ?? s.client), status: "online" as const };
      openConversation(conv ? { ...conv, counterpart: requester, open_session: s } : { id: s.conversation_id, counterpart: requester, unread: 0, open_session: s, created_at: s.requested_at });
      return;
    }
    const sameConversation = s.conversation_id === activeConvId || (!activeConvId && role === "client" && s.psychic.slug === slug);
    if (!sameConversation) return;
    if (!activeConvId) setActiveConvId(s.conversation_id);
    if (role === "psychic" && isOpen(s)) setCounterpart((c) => (c && c.id === s.client.id && c.status !== "online" ? { ...c, status: "online" } : c));
    setSnap(snapshot(s));
    if (data.summary) setSummary(data.summary);
    if (type === "session.started") setLowBalance(null);
    if (!isOpen(s)) {
      setTyping(false);
      requestKey.current = newId();
    }
  };
  useRealtimeEvents<SessionEvent>(SESSION_EVENTS, onSession);

  // Automatic re-join after a reconnect returns the fresh (often resumed) session.
  useRealtimeEvent<ChatSession>("session.rejoined", (view) => {
    if (view?.id && view.id === session?.id) setSnap(snapshot(view));
  });

  useRealtimeEvent<SessionBilledEvent>("session.billed", (b) => {
    setSnap((cur) =>
      cur && cur.session.id === b.session_id
        ? {
            session: {
              ...cur.session,
              billed_minutes: b.minute,
              active_seconds: b.active_seconds,
              next_billing_at: b.next_billing_at,
              server_time: new Date().toISOString(),
              // Advisors: live earnings so far.
              ...(b.sponsored ? { sponsored: true } : {}),
              ...(typeof b.psychic_earnings === "number"
                ? { psychic_earnings: b.psychic_earnings, psychic_earnings_cents: b.psychic_earnings_cents, currency: b.currency ?? cur.session.currency }
                : {}),
            },
            receivedAt: Date.now(),
          }
        : cur,
    );
    if (role === "psychic" && typeof b.psychic_earnings !== "number" && b.session_id === session?.id) {
      // Older servers do not send earnings with the minute: read them.
      chatApi.session(b.session_id).then(
        (view) => setSnap((cur) => (cur && cur.session.id === view.id ? snapshot(view) : cur)),
        (err) => logger.debug("earnings refresh failed", { code: String(toApiError(err).code) }),
      );
    }
    if (typeof b.minutes_remaining === "number" && b.minutes_remaining >= 2) setLowBalance(null);
  });
  useRealtimeEvent<{ session_id: string; minutes_remaining: number }>("session.low_balance", (d) => {
    if (d.session_id === session?.id) setLowBalance(d.minutes_remaining);
  });

  useRealtimeEvent<{ message: ChatMessage }>("message.created", ({ message }) => {
    if (message.conversation_id === activeConvId) {
      receive([message]);
      if (message.sender_id && message.sender_id !== meId) {
        setTyping(false);
        if (message.kind !== "system") pushLog([{ id: message.id, text: describeIncoming(message) }]);
      }
    } else if (!activeConvId && role === "client" && liveSession?.conversation_id === message.conversation_id) {
      setActiveConvId(message.conversation_id);
    }
    if (shouldToastOtherConversation({ message, meId, activeConversationId: activeConvId, hidden: document.visibilityState === "hidden", permission: "unsupported" })) {
      // Advisors juggle clients: say who wrote, with a way to open that chat.
      const row = inbox.items.find((c) => c.id === message.conversation_id);
      toast.show({
        tone: "info",
        message: row ? text("New message from {value0}", { value0: role === "psychic" ? nameFor(row.counterpart.id, row.counterpart.name) : row.counterpart.name }) : text("New message"),
        actionLabel: row ? text("Open") : undefined,
        onAction: row ? () => openConversation(row) : undefined,
      });
    }
    if (message.sender_id === meId && message.client_msg_id) outbox.confirm([message.client_msg_id]);
    const next = withMessage(inbox.items, message, { meId, activeId: activeConvId });
    if (next) inbox.patch(() => next);
    else inbox.refresh();
  });

  // `liked_by` is authoritative (§23.3); a message that is not loaded is ignored by the timeline.
  useRealtimeEvent<MessageLikedEvent>("message.liked", (like) => {
    if (like?.message_id) thread.applyLikes(like);
  });

  useRealtimeEvent<{ session_id: string; user_id: string; typing: boolean }>("typing", (d) => {
    if (d.session_id !== session?.id || d.user_id === meId) return;
    setTyping(d.typing);
    if (typingTimer.current) clearTimeout(typingTimer.current);
    if (d.typing) typingTimer.current = setTimeout(() => setTyping(false), 5000);
  });

  useRealtimeEvent<MessageReadEvent>("message.read", (d) => {
    if (d.conversation_id !== activeConvId || d.reader_id === meId) return;
    let seq = d.read_seq ?? 0;
    // Older receipts name a message instead of a seq.
    if (!seq && d.message_id) seq = timeline.messages.find((m) => m.id === d.message_id)?.seq ?? 0;
    if (seq) setReads((r) => mergeReads(r, d.conversation_id, undefined, seq));
  });

  // Advisors: live online/offline presence of their clients (§9.4).
  useRealtimeEvent<{ user_id: string; online: boolean }>("presence.updated", (d) => {
    if (role !== "psychic" || !d?.user_id) return;
    const status = d.online ? "online" : "offline";
    inbox.patch((list) => list.map((c) => (c.counterpart.id === d.user_id ? { ...c, counterpart: { ...c.counterpart, status } } : c)));
    setCounterpart((c) => (c && c.id === d.user_id ? { ...c, status } : c));
  });

  useRealtimeEvent<{ slug: string; status: Psychic["status"] }>("psychic.status_changed", (d) => {
    if (role !== "client") return;
    inbox.patch((list) => list.map((c) => (c.counterpart.slug === d.slug ? { ...c, counterpart: { ...c.counterpart, status: d.status } } : c)));
    if (d.slug === psychic?.slug) {
      setPsychic((p) => (p ? { ...p, status: d.status } : p));
      setCounterpart((c) => (c ? { ...c, status: d.status } : c));
    }
  });

  // After a reconnect (or when the server lost events): fetch what was missed
  // once over REST, and refresh session state when nothing was replayed.
  const resync = React.useCallback(
    (full: boolean) => {
      fillGaps();
      inbox.refresh();
      if (!full) return;
      // Events (likes among them) may have been lost: the newest page is the truth again.
      refreshNewest();
      if (role === "client") setThreadNonce((n) => n + 1);
      else if (openSessionId)
        chatApi.session(openSessionId).then(
          (view) => setSnap(snapshot(view)),
          (err) => logger.debug("session refresh failed", { code: String(toApiError(err).code) }),
        );
    },
    [fillGaps, refreshNewest, inbox, role, openSessionId],
  );
  useRealtimeEvent("resync_required", () => resync(true));
  useRealtimeEvent<{ reconnected?: boolean; resumed?: boolean }>("hello", (d) => {
    if (d.reconnected) resync(!d.resumed);
  });

  // --- read receipts (by seq) ------------------------------------------------
  const conversationRow = inbox.items.find((c) => c.id === activeConvId);
  const myRead = Math.max(reads.conv === activeConvId ? reads.mine : 0, conversationRow?.read_seq ?? 0);
  const peerRead = Math.max(reads.conv === activeConvId ? reads.peer : 0, conversationRow?.peer_read_seq ?? 0);
  const newestIncoming = newestIncomingSeq(timeline.messages, meId);
  React.useEffect(() => {
    if (!activeConvId || !visible || !atBottom || newestIncoming <= myRead) return;
    const conv = activeConvId;
    const seq = newestIncoming;
    const timer = setTimeout(() => {
      const sent = realtime
        ? realtime.request<{ read_seq?: number; unread?: number }>("message.read", { conversation_id: conv, seq })
        : chatApi.markReadSeq(conv, seq);
      sent.then(
        (res) => {
          const readSeq = res?.read_seq ?? seq;
          setReads((r) => mergeReads(r, conv, readSeq, undefined));
          inbox.patch((list) => list.map((c) => (c.id === conv ? { ...c, unread: res?.unread ?? 0, read_seq: Math.max(c.read_seq ?? 0, readSeq) } : c)));
        },
        (err) => logger.debug("mark read failed", { code: String(toApiError(err).code) }),
      );
    }, READ_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [activeConvId, visible, atBottom, newestIncoming, myRead, realtime, inbox]);

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
        inbox.refresh();
      }
    }
  }

  // Stable identity: the composer stops its typing indicator whenever this changes.
  const typingSessionId = session?.id;
  const onTyping = React.useCallback(
    (isTyping: boolean) => {
      if (typingSessionId) realtime?.send("typing", { session_id: typingSessionId, typing: isTyping });
    },
    [realtime, typingSessionId],
  );

  function send(body: string, replyTo?: ReplyPreview) {
    if (!availability.enabled) return;
    if (outbox.send(body, replyTo)) setScrollToken((n) => n + 1);
  }

  /** Upload an image message with an optimistic pending/failed/retry flow (kept in memory: files are not persisted). */
  async function sendImage(file: File, caption: string, retryOf?: UiMessage, knownSize?: { width: number; height: number }, replyTo?: ReplyPreview) {
    const conversationId = retryOf?.conversation_id ?? activeConvId;
    if (!conversationId || !availability.enabled) return;
    const cmid = retryOf?.client_msg_id ?? newClientMsgId();
    // A retry keeps the reply it was written with.
    const quoted = retryOf ? retryOf.reply_to : replyTo;
    let local: UiMessage;
    if (retryOf) local = { ...retryOf, pending: "sending", failure: undefined, failureDetail: undefined };
    else {
      const size = knownSize ?? (await imageSize(file));
      local = {
        id: cmid,
        client_msg_id: cmid,
        conversation_id: conversationId,
        session_id: liveSession?.id,
        sender_id: meId,
        kind: "image",
        body: caption,
        attachments: [{ id: `local-${cmid}`, content_type: file.type, width: size?.width ?? 0, height: size?.height ?? 0, bytes: file.size }],
        ...(quoted ? { reply_to: quoted } : {}),
        created_at: new Date().toISOString(),
        pending: "sending",
        localFile: file,
      };
    }
    setImages((list) => mergeMessages(list, [local]));
    setScrollToken((n) => n + 1);
    try {
      // The API routes it into the live chat or sends it outside one; it deduplicates on the client id, so transient failures retry.
      const { message, allowance: fresh } = await chatApi.uploadToConversation(conversationId, file, caption || undefined, cmid, undefined, quoted?.id);
      const attachment = message.attachments?.[0];
      // Show our own copy instead of downloading it again (GIFs are stored as their first frame).
      if (attachment && file.type !== "image/gif") seedAttachment(attachment.id, file);
      receive([message]);
      allowance.update(fresh);
      setImages((list) => list.filter((x) => x.client_msg_id !== cmid));
    } catch (err) {
      const e = toApiError(err);
      // The server explains image problems itself (dimensions, decoding); show that as is.
      const detail = e.code === "validation_failed" ? e.fields.image : undefined;
      setImages((list) => list.map((x) => (x.client_msg_id === cmid ? { ...x, pending: "failed", failure: String(e.code), failureDetail: detail } : x)));
      if (String(e.code).startsWith("off_session_") || e.code === "recipient_unavailable") allowance.reload();
      if (e.code !== "network_error" && e.code !== "timeout") {
        if (detail) toast.show({ tone: "error", message: detail });
        else toast.showError(e);
        announce(text("A message was not delivered. You can retry it."), "assertive");
      }
      logger.warn("image upload failed", e, { code: String(e.code) });
    }
  }

  function submit({ body, file, size, replyTo }: ComposerSubmit) {
    if (file) void sendImage(file, body, undefined, size, replyTo);
    else send(body, replyTo);
  }

  function retry(m: UiMessage) {
    if (m.kind === "image") {
      if (!availability.enabled) toast.show({ tone: "info", message: unavailableReason });
      else if (m.localFile) void sendImage(m.localFile, m.body, m);
      return;
    }
    if (!availability.enabled) {
      // Say why instead of failing again (e.g. the daily limit is used up).
      toast.show({ tone: "info", message: unavailableReason });
      return;
    }
    if (m.client_msg_id) outbox.retry(m.client_msg_id);
  }

  function discard(m: UiMessage) {
    if (!m.client_msg_id) return;
    if (m.kind === "image") setImages((list) => list.filter((x) => x.client_msg_id !== m.client_msg_id));
    else outbox.discard(m.client_msg_id);
  }

  const isSaved = (m: ChatMessage) => savedOverride[m.id] ?? Boolean(m.saved);
  async function setBookmark(id: string, value: boolean) {
    try {
      if (value) await chatApi.save(id);
      else await chatApi.unsave(id);
      setSavedOverride((o) => ({ ...o, [id]: value }));
      // Newest saves come first: start from the first page again.
      if (value) saved.reset();
      else saved.reload();
    } catch (err) {
      toast.showError(err);
    }
  }
  function toggleBookmark(m: UiMessage) {
    if (m.pending || (m.kind !== "text" && m.kind !== "image")) return;
    void setBookmark(m.id, !isSaved(m));
  }

  // --- replies and likes (§23.2, §23.3) ---------------------------------------
  const activeReply = (activeConvId && replyTargets[activeConvId]) || null;
  const reply = (m: UiMessage) => {
    if (!isActionable(m) || !m.conversation_id) return;
    setReplyTargets((cur) => ({ ...cur, [m.conversation_id]: replyPreviewOf(m) }));
    composerRef.current?.focus();
  };
  const cancelReply = () => {
    if (!activeConvId) return;
    setReplyTargets((cur) => {
      if (!(activeConvId in cur)) return cur;
      const { [activeConvId]: _gone, ...rest } = cur;
      void _gone;
      return rest;
    });
  };

  const { override: likeOverride, toggle: toggleLike } = useMessageLikes({
    meId,
    messages: timeline.messages,
    applyLikes: thread.applyLikes,
    onFailed: (e, retry) => {
      toast.show({ tone: "error", message: text(errorMessage(e)), actionLabel: text("Try again"), onAction: retry });
      track("error_displayed", { code: String(e.code), area: "chat_like" });
    },
  });

  // Quote → original: scroll to it, loading older pages (bounded) when it is not on screen yet.
  const { jumpTo, jumpingTo } = useJumpToMessage({
    scrollTo: (id) => feedRef.current?.scrollToMessage(id) ?? false,
    loadOlder: thread.loadOlder,
    conversationId: activeConvId,
    onSearching: () => announce(text("Loading the original message…")),
    onFailed: (reason, retry) =>
      toast.show({
        tone: "error",
        message: text(reason === "unavailable" ? "The original message is no longer available." : "Couldn't load the original message."),
        ...(retry ? { actionLabel: text("Try again"), onAction: retry } : {}),
      }),
  });

  // --- render ------------------------------------------------------------------
  const localMessages = React.useMemo(() => {
    const queued = outbox.entries.filter((e) => e.conversation_id === activeConvId).map(outboxMessage);
    return [...queued, ...images.filter((m) => m.conversation_id === activeConvId)];
  }, [outbox.entries, images, activeConvId]);
  const merged = React.useMemo(() => mergeMessages(timeline.messages, localMessages), [timeline.messages, localMessages]);
  // Optimistic likes show until the API answers (§23.3).
  const messages = React.useMemo(() => withLikeOverrides(merged, likeOverride), [merged, likeOverride]);
  const hasSent = React.useMemo(() => messages.some((m) => m.sender_id === meId && m.kind !== "system"), [messages, meId]);
  const savedIds = React.useMemo(() => {
    const ids = new Set<string>();
    for (const m of messages) if (isSaved(m)) ids.add(m.id);
    return ids;
    // isSaved reads savedOverride.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [messages, savedOverride]);

  const messageActions: MessageActions = {
    enabled: true,
    viewerId: meId,
    replyTarget: activeReply,
    reply,
    cancelReply,
    toggleLike,
    jumpTo: (quote) => void jumpTo(quote),
    jumpingTo,
    nameOf: (senderId) => (senderId && senderId === meId ? t("chatroom.feed.you") : counterpartDisplay),
  };

  const rateView = sessionRate(session, psychic?.rate_per_minute ?? 0);
  const earnings = role === "psychic" ? sessionEarnings(session) : null;
  const earned = useEarningsLabel(earnings && session && session.status !== "requested" ? earnings : null);

  if (status !== "authenticated" || !user) {
    return (
      <main className="chat-workspace grid min-h-0 flex-1 place-items-center px-8">
        <p className="text-sm" role="status">
          {text("Loading your conversations…")}
        </p>
      </main>
    );
  }

  const deadline = session?.status === "disconnected" ? session.grace_expires_at : session?.expires_at;
  const secondsLeft = secondsUntil(deadline, snap);
  const elapsed = elapsedSeconds(snap);
  const returnPath = `/${locale}/chatroom${slug ? `?reader=${slug}` : ""}`;
  const shownSession = session ?? summary?.session ?? null;
  const report: ReportIssueTarget | null =
    role === "client" && activeConvId && counterpart
      ? {
          conversationId: activeConvId,
          sessionId: shownSession && shownSession.conversation_id === activeConvId ? shownSession.id : undefined,
          advisorName: counterpart.name,
          seq: timeline.messages.reduce((max, m) => Math.max(max, m.seq ?? 0), 0) || undefined,
        }
      : null;
  // Phones: unread elsewhere (other conversations) or unread notifications dot the menu button.
  const unreadHere = conversationRow?.unread ?? 0;
  const activity = Math.max(0, chatUnread.count - unreadHere) > 0 || notifications.unread > 0;
  const free = wallet?.free_minutes ?? 0;
  const freeChat = freeMode || Boolean(session?.sponsored);
  const a = allowance.allowance;
  // Outside a live chat the note shows the allowance instead of the rate.
  const offSessionNote = !inLiveChat && a && a.enabled && a.eligible && !a.live ? a : null;
  const offlineAdvisor = Boolean(counterpart && counterpart.status && counterpart.status !== "online");
  const disabledAction = availability.enabled ? null : availability.canRetry ? (
    <button type="button" className="underline" onClick={allowance.reload}>
      {text("Try again")}
    </button>
  ) : availability.canStartChat && slug ? (
    <Button variant="cta" size="sm" onClick={() => void startChat()} disabled={busy || offlineAdvisor} data-testid="composer-start-chat">
      {text("Start a live chat")}
    </Button>
  ) : null;
  const note = (
    <ComposerNote
      offSession={offSessionNote}
      role={role}
      free={freeChat}
      freeMinutes={free}
      balance={wallet?.balance ?? 0}
      rate={rateView.rate > 0 ? rateView : null}
      advisorLabel={t("chatroom.composer.label")}
    />
  );

  const listProps = {
    conversations: inbox.items,
    viewerRole: role,
    activeId: activeConvId,
    onSelect: selectConversation,
    status: inbox.status,
    error: inbox.error,
    onRetry: inbox.refresh,
    hasMore: inbox.hasMore,
    loadingMore: inbox.loadingMore,
    moreError: inbox.moreError,
    onLoadMore: inbox.loadMore,
  } as const;
  const removeSaved = (id: string) => void setBookmark(id, false);
  // Advisors get the client panel as a column (xl+) or as tabs in the details sheet.
  const renderPanel = (variant: "column" | "tabs") =>
    counterpart ? (
      role === "psychic" ? (
        <ClientPanel
          key={activeConvId ?? "none"}
          variant={variant}
          conversationId={activeConvId}
          counterpart={counterpart}
          online={counterpart.status ? counterpart.status !== "offline" : undefined}
          saved={saved}
          onRemove={removeSaved}
        />
      ) : (
        <ReaderPanel counterpart={counterpart} psychic={psychic} saved={saved} onRemove={removeSaved} />
      )
    ) : null;

  const queued = outbox.entries.filter((e) => e.state !== "failed").length;
  const connectionBanner =
    connection.state === "offline" ? (
      <>
        {text("You are offline. Messages will be sent when your connection returns.")}
        {queued ? <> {text("{value0} waiting to send.", { value0: queued })}</> : null}
      </>
    ) : connection.state === "reconnecting" || connection.state === "connecting" ? (
      <>
        {text("Reconnecting to live chat…")}
        {queued ? <> {text("{value0} waiting to send.", { value0: queued })}</> : null}{" "}
        {connection.state === "reconnecting" ? (
          <button type="button" className="underline" onClick={() => realtime?.reconnect()}>
            {text("Retry now")}
          </button>
        ) : null}
      </>
    ) : null;

  return (
    <MessageActionsProvider value={messageActions}>
    <main
      aria-label={text("Chatroom")}
      data-role={role}
      className="chat-workspace grid min-h-0 flex-1 gap-4 px-8 lg:grid-cols-[260px_minmax(0,1fr)] xl:grid-cols-[260px_minmax(0,1fr)_280px] 2xl:gap-6 2xl:px-12 2xl:grid-cols-[300px_minmax(0,1fr)_320px] min-[120rem]:gap-8 min-[120rem]:px-16 min-[120rem]:grid-cols-[360px_minmax(0,1fr)_380px]"
    >
      <aside className="chat-inbox-column hidden min-h-0 overflow-hidden rounded-xl border lg:block" aria-label={t("chatroom.inbox.title")}>
        <ConversationList {...listProps} />
      </aside>
      {/* overflow-clip (not hidden): the column can never be scrolled by scrollIntoView or focus. */}
      <section className="chat-column flex min-h-0 flex-col overflow-clip rounded-xl border" {...drop.handlers} data-dragging={drop.dragging || undefined}>
        {connectionBanner ? (
          <p className="chat-connection-banner" role="status" data-testid="connection-banner" data-state={connection.state}>
            {connectionBanner}
          </p>
        ) : null}
        {counterpart ? (
          <>
            <ChatHeader
              counterpart={counterpart}
              conversationId={activeConvId}
              viewerRole={role}
              session={session}
              elapsedSeconds={elapsed}
              rate={rateView.rate > 0 ? rateView : null}
              earnings={earnings}
              onOpenInbox={() => setInboxOpen(true)}
              onOpenDetails={() => setDetailsOpen(true)}
              activity={activity}
              report={report}
              ending={busy}
              onEnd={() =>
                session &&
                void act(() => chatApi.end(session.id), (sum) => {
                  setSummary(sum);
                  setSnap(snapshot(sum.session));
                })
              }
            />
            <SessionTools role={role} wallet={wallet} free={freeChat} earned={earned} returnPath={returnPath} onOpenDetails={() => setDetailsOpen(true)} report={report} />
            {otherSession && role === "client" ? (
              <p className="chat-connection-banner" role="status">
                {text("You already have a live chat with {value0}.", { value0: otherSession.psychic.name })}{" "}
                <button type="button" className="underline" onClick={() => otherSession.psychic.slug && setSlug(otherSession.psychic.slug)}>
                  {text("Go to that chat")}
                </button>
              </p>
            ) : null}
            {lowBalance !== null && session?.status === "active" && !freeChat ? (
              <p className="chat-connection-banner" role="alert">
                {text("About {value0} minutes of credit left.", { value0: lowBalance })}{" "}
                <a className="underline" href={`/${locale}/credits?next=${encodeURIComponent(returnPath)}`} target="_blank" rel="noopener">
                  {text("Top up credits")}
                </a>
              </p>
            ) : null}
            {threadError || thread.error ? (
              <div className="p-6 text-sm" role="alert">
                <p>{text(errorMessage((threadError ?? thread.error)!))}</p>
                <button
                  type="button"
                  className="mt-2 underline"
                  onClick={() => {
                    if (threadError) setThreadNonce((n) => n + 1);
                    thread.reload();
                  }}
                >
                  {text("Try again")}
                </button>
              </div>
            ) : (
              <ChatFeed
                ref={feedRef}
                messages={messages}
                meId={meId}
                authorName={counterpartDisplay}
                typing={typing && session?.status === "active"}
                savedIds={savedIds}
                peerReadSeq={peerRead}
                status={thread.status}
                hasOlder={timeline.hasOlder}
                loadingOlder={thread.loadingOlder}
                olderError={thread.olderError}
                onLoadOlder={() => void thread.loadOlder()}
                onBookmark={toggleBookmark}
                onRetry={retry}
                onDiscard={discard}
                connected={connection.state === "open"}
                announcements={log}
                scrollToken={scrollToken}
                onAtBottomChange={setAtBottom}
              >
                <SessionPanel
                  role={role}
                  counterpart={counterpart}
                  session={session}
                  summary={summary}
                  wallet={wallet}
                  rate={rateView.rate}
                  rateView={rateView}
                  secondsLeft={secondsLeft}
                  busy={busy}
                  error={actionError}
                  returnPath={returnPath}
                  free={freeChat}
                  onStart={() => void startChat()}
                  onCancel={() => session && void act(() => chatApi.cancel(session.id), (s) => setSnap(snapshot(s)))}
                  onAccept={() => session && void act(() => chatApi.accept(session.id), (s) => setSnap(snapshot(s)))}
                  onDecline={() => session && void act(() => chatApi.decline(session.id), (s) => setSnap(snapshot(s)))}
                  onDismissSummary={() => setSummary(null)}
                  onReviewed={(review: Review) => setSummary((sum) => (sum ? { ...sum, review, can_review: false } : sum))}
                  onAdvisorAvailable={() => {
                    setPsychic((p) => (p ? { ...p, status: "online" } : p));
                    setCounterpart((c) => (c ? { ...c, status: "online" } : c));
                  }}
                />
              </ChatFeed>
            )}
            <ChatPushPrompt userId={meId} hasSent={hasSent} role={role} />
            <MessageComposer
              ref={composerRef}
              userId={meId}
              draftId={activeConvId ?? (slug ? `reader:${slug}` : null)}
              draftFallbackId={activeConvId && slug ? `reader:${slug}` : null}
              onSend={submit}
              onTyping={onTyping}
              disabled={!availability.enabled}
              disabledReason={unavailableReason}
              disabledAction={disabledAction}
              note={note}
            />
            {drop.dragging ? (
              <div className="chat-drop-overlay" aria-hidden data-testid="drop-overlay">
                {text("Drop the image to send it to {value0}", { value0: counterpartDisplay })}
              </div>
            ) : null}
          </>
        ) : inbox.status === "error" && inbox.error ? (
          // No conversation to show because the inbox failed: retry from here (phones have no inbox column).
          <>
            <ChatroomTopBar onOpenInbox={() => setInboxOpen(true)} activity={activity} />
            <div className="grid flex-1 place-items-center p-6">
              <ErrorState error={inbox.error} onRetry={inbox.refresh} title={text("Your conversations could not be loaded.")} className="w-full max-w-md bg-cream/60" />
            </div>
          </>
        ) : inbox.status === "loading" ? (
          <>
            <ChatroomTopBar onOpenInbox={() => setInboxOpen(true)} activity={activity} />
            <div className="grid flex-1 place-items-center p-8 text-center text-sm">
              <p role="status">{text("Loading your conversations…")}</p>
            </div>
          </>
        ) : (
          <>
            <ChatroomTopBar onOpenInbox={() => setInboxOpen(true)} activity={activity} />
            <div className="grid flex-1 place-items-center p-8 text-center text-sm">
              <div className="flex flex-col items-center gap-3">
                <p>{text(role === "psychic" ? "Chat requests from clients appear here while you are online." : "Choose an advisor to start a conversation.")}</p>
                {role === "client" ? (
                  <Button asChild variant="cta" className="px-5">
                    <Link href={`/${locale}/psychics`}>{text("Browse all advisors")}</Link>
                  </Button>
                ) : null}
              </div>
            </div>
          </>
        )}
      </section>
      <aside className={`chat-detail-column hidden min-h-0 overflow-y-auto rounded-xl border xl:block`}>{renderPanel("column")}</aside>
      <Sheet open={inboxOpen} onOpenChange={setInboxOpen}>
        <SheetContent side="left" className="chat-switcher-sheet gap-0 p-0" data-testid="chat-switcher">
          <SheetHeader className="pb-0">
            <SheetTitle>{t("chatroom.inbox.title")}</SheetTitle>
          </SheetHeader>
          <SwitcherAccount role={role} returnPath={returnPath} free={freeChat} onNavigate={() => setInboxOpen(false)} />
          <SwitcherHeader
            role={role}
            counterpart={counterpart}
            rate={rateView.rate > 0 ? rateView : null}
            liveWith={role === "client" && liveSession && liveSession.status !== "requested" ? (counterpart?.name ?? null) : null}
          />
          <div className="relative min-h-0 flex-1">
            <ConversationList {...listProps} variant="cards" excludeId={activeConvId} className="absolute inset-0 h-auto" />
          </div>
          <div className="chat-switcher-foot">
            <SwitcherQuickLinks onNavigate={() => setInboxOpen(false)} />
            <Button asChild variant="ghost" className="chat-leave-button">
              <Link href={role === "client" ? `/${locale}/psychics` : `/${locale}/profile`}>{text("Leave Chatroom")}</Link>
            </Button>
          </div>
        </SheetContent>
      </Sheet>
      <Sheet open={detailsOpen} onOpenChange={setDetailsOpen}>
        <SheetContent side="right" className="chat-details-sheet w-[300px] gap-0 overflow-y-auto p-0">
          <SheetHeader className="pb-0">
            <SheetTitle className="sr-only">{text(counterpartDisplay)}</SheetTitle>
          </SheetHeader>
          {renderPanel("tabs")}
        </SheetContent>
      </Sheet>
      <AlertDialog open={Boolean(pendingSwitch)} onOpenChange={(open) => !open && setPendingSwitch(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{text("Switch to {value0}?", { value0: pendingSwitch?.counterpart.name ?? "" })}</AlertDialogTitle>
            <AlertDialogDescription>
              {text("Confirming ends your paid session with {value0}. The next advisor's rate applies; your free trial does not restart.", {
                value0: counterpart?.name ?? "",
              })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{text("Keep chatting")}</AlertDialogCancel>
            <AlertDialogAction onClick={() => void confirmSwitch()}>{text("End chat and switch")}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
      {initialReview && role === "client" ? (
        <ReviewDialog
          key={initialReview}
          sessionId={initialReview}
          open={reviewOpen}
          onOpenChange={(open) => {
            if (open) return;
            setDismissedReview(initialReview);
            router.replace(`/${locale}/chatroom${slug ? `?reader=${slug}` : ""}`, { scroll: false });
          }}
        />
      ) : null}
    </main>
    </MessageActionsProvider>
  );
}

/** Read positions only move forward (receipts can arrive out of order). */
function mergeReads(
  r: { conv: string | null; mine: number; peer: number },
  conv: string,
  mine: number | undefined,
  peer: number | undefined,
): { conv: string | null; mine: number; peer: number } {
  const base = r.conv === conv ? r : { conv, mine: 0, peer: 0 };
  const next = { conv, mine: Math.max(base.mine, mine ?? 0), peer: Math.max(base.peer, peer ?? 0) };
  return next.mine === r.mine && next.peer === r.peer && r.conv === conv ? r : next;
}
