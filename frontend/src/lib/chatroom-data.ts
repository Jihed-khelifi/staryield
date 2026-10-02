/**
 * UI-only fixtures for the chatroom screen. Every string that a reader sees
 * lives in the dictionaries — this file only holds the keys and the numbers.
 */

export type ReaderStatus = "online" | "away" | "busy";

export type Conversation = {
  id: string;
  name: string;
  /** Letter shown in the square avatar, matching the Figma design. */
  initial: string;
  status: ReaderStatus;
  /** Dictionary path for the last-message preview. */
  previewKey: string;
  unread: number;
};

export type Reader = {
  id: string;
  name: string;
  initial: string;
  status: ReaderStatus;
  rating: number;
  years: number;
  bioKey: string;
  freeMinutes: number;
  creditsPerMinute: number;
};

export type ChatMessage = {
  id: string;
  author: "reader" | "visitor";
  /** Dictionary path for the message body. */
  bodyKey?: string;
  /** Literal body, used for messages typed in the composer. */
  body?: string;
  /** English source copy for bundled demo messages; user-written bodies stay literal. */
  fixtureKey?: string;
  /** Fixed UTC instant so server and client format identically. */
  sentAt: string;
  /** Blurred paid answer with a "See the answer" call to action. */
  locked?: boolean;
};

export const conversations: Conversation[] = [
  {
    id: "ramone",
    name: "Ramone",
    initial: "R",
    status: "online",
    previewKey: "readers.ramone.preview",
    unread: 1,
  },
  {
    id: "theo",
    name: "Theo",
    initial: "C",
    status: "online",
    previewKey: "readers.theo.preview",
    unread: 3,
  },
  {
    id: "solomon",
    name: "Solomon",
    initial: "S",
    status: "busy",
    previewKey: "readers.solomon.preview",
    unread: 0,
  },
];

export const readers: Record<string, Reader> = {
  ramone: {
    id: "ramone",
    name: "Ramone",
    initial: "R",
    status: "online",
    rating: 4.8,
    years: 8,
    bioKey: "readers.ramone.bio",
    freeMinutes: 3,
    creditsPerMinute: 40,
  },
  theo: {
    id: "theo",
    name: "Theo",
    initial: "C",
    status: "online",
    rating: 4.9,
    years: 5,
    bioKey: "readers.theo.bio",
    freeMinutes: 3,
    creditsPerMinute: 40,
  },
  solomon: {
    id: "solomon",
    name: "Solomon",
    initial: "S",
    status: "busy",
    rating: 4.8,
    years: 4,
    bioKey: "readers.solomon.bio",
    freeMinutes: 3,
    creditsPerMinute: 40,
  },
};

export const conversationThread: ChatMessage[] = [
  {
    id: "m1",
    author: "visitor",
    bodyKey: "conversation.m1",
    sentAt: "2026-07-30T01:13:00Z",
  },
  {
    id: "m2",
    author: "reader",
    bodyKey: "conversation.m2",
    sentAt: "2026-07-30T01:13:00Z",
  },
  {
    id: "m3",
    author: "reader",
    bodyKey: "conversation.m3",
    sentAt: "2026-07-30T01:14:00Z",
  },
  {
    id: "m4",
    author: "reader",
    bodyKey: "conversation.m4",
    sentAt: "2026-07-30T01:15:00Z",
  },
  {
    id: "m5",
    author: "visitor",
    bodyKey: "conversation.m5",
    sentAt: "2026-07-30T01:15:00Z",
  },
  {
    id: "m6",
    author: "reader",
    bodyKey: "conversation.m6",
    sentAt: "2026-07-30T01:15:00Z",
  },
  {
    id: "m7",
    author: "reader",
    bodyKey: "conversation.m7",
    sentAt: "2026-07-30T01:17:00Z",
    locked: true,
  },
];

/** Session length shown in the chat header (00:32:53 in the design). */
export const initialSessionSeconds = 32 * 60 + 53;
