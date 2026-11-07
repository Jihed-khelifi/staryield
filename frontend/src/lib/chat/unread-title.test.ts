import { describe, expect, it } from "vitest";

import { stripUnreadPrefix, withUnreadPrefix } from "@/lib/chat/unread-title";

describe("unread title", () => {
  it("prefixes and caps at 99+", () => {
    expect(withUnreadPrefix("Chatroom", 3)).toBe("(3) Chatroom");
    expect(withUnreadPrefix("Chatroom", 250)).toBe("(99+) Chatroom");
  });
  it("never stacks prefixes and restores the title", () => {
    expect(withUnreadPrefix("(2) Chatroom", 5)).toBe("(5) Chatroom");
    expect(withUnreadPrefix("(2) Chatroom", 0)).toBe("Chatroom");
    expect(stripUnreadPrefix("(99+) Chatroom")).toBe("Chatroom");
    expect(stripUnreadPrefix("(not a count) x")).toBe("(not a count) x");
  });
});
