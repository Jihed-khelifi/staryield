// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen, within } from "@testing-library/react";

import type { Conversation } from "@/lib/api/types";
import { renderWithI18n } from "@/test/render";
import { ConversationList } from "@/components/chat/conversation-list";

afterEach(cleanup);

const row = (last: Partial<NonNullable<Conversation["last_message"]>>, over: Partial<Conversation> = {}): Conversation => ({
  id: "c1",
  counterpart: { id: "p1", role: "psychic", name: "Theo", status: "online" },
  unread: 1,
  created_at: "2026-11-19T10:00:00Z",
  last_message: { id: "m1", conversation_id: "c1", kind: "text", body: "Visible words", created_at: "2026-11-19T10:00:00Z", ...last },
  ...over,
});
const list = (items: Conversation[]) => renderWithI18n(<ConversationList conversations={items} activeId={null} onSelect={vi.fn()} />);

describe("inbox preview of a locked message (§26.1)", () => {
  it("shows 'Hidden message' with a lock, never the text", () => {
    list([row({ locked: true, hold_id: "h1", body: "", body_length: 20 })]);
    const preview = within(screen.getByTestId("inbox-row")).getByTestId("hidden-message-label");
    expect(preview.textContent).toContain("Hidden message");
    expect(preview.querySelector("svg")).toBeTruthy();
    expect(screen.getByTestId("inbox-row").textContent).not.toContain("Visible words");
  });

  it("even a body a buggy server sent is not shown", () => {
    list([row({ locked: true, body: "SECRET PREVIEW" })]);
    expect(screen.getByTestId("inbox-row").textContent).not.toContain("SECRET PREVIEW");
  });

  it("a readable last message previews as before", () => {
    list([row({})]);
    expect(screen.getByTestId("inbox-row").textContent).toContain("Visible words");
    expect(screen.queryByTestId("hidden-message-label")).toBeNull();
  });

  it("a revealed message (locked: false) previews its text", () => {
    list([row({ locked: false })]);
    expect(screen.getByTestId("inbox-row").textContent).toContain("Visible words");
  });
});
