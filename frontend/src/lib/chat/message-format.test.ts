import { describe, expect, it } from "vitest";

import { emojiOnlyCount, insertAtCaret, isEmojiOnly, linkify, safeHref, trimUrl } from "@/lib/chat/message-format";

const links = (text: string) => linkify(text).filter((t) => t.type === "link");

describe("linkify", () => {
  it("keeps the text intact when concatenated", () => {
    const text = "See https://example.com/a?b=1#c, then www.staryield.net.\n  Indented\tline";
    expect(linkify(text).map((t) => t.value).join("")).toBe(text);
  });

  it("links http(s) URLs and www. hosts (as https)", () => {
    expect(links("go to http://example.com now")).toEqual([{ type: "link", value: "http://example.com", href: "http://example.com/" }]);
    expect(links("visit www.staryield.net")).toEqual([{ type: "link", value: "www.staryield.net", href: "https://www.staryield.net/" }]);
    expect(links("HTTPS://EXAMPLE.COM/Path")[0].href).toBe("https://example.com/Path");
  });

  it.each([
    ["Look: https://example.com.", "https://example.com"],
    ["Really? https://example.com/a!", "https://example.com/a"],
    ["(see https://example.com/page)", "https://example.com/page"],
    ["https://en.wikipedia.org/wiki/Tarot_(game) is nice", "https://en.wikipedia.org/wiki/Tarot_(game)"],
    ["(https://en.wikipedia.org/wiki/Tarot_(game))", "https://en.wikipedia.org/wiki/Tarot_(game)"],
    ["https://example.com/path_(x)).", "https://example.com/path_(x)"],
    ['"https://example.com/q?x=1"', "https://example.com/q?x=1"],
    ["https://example.com/a, https://example.com/b;", "https://example.com/a"],
    ["[https://example.com/x]", "https://example.com/x"],
    ["<https://example.com/x>", "https://example.com/x"],
    ["https://example.com/a…", "https://example.com/a"],
  ])("trims trailing punctuation in %s", (text, url) => {
    expect(links(text)[0].value).toBe(url);
  });

  it("finds several links", () => {
    expect(links("https://a.com and www.b.org/x").map((l) => l.value)).toEqual(["https://a.com", "www.b.org/x"]);
  });

  it.each([
    ["javascript:alert(1)"],
    ["javascript://example.com/%0Aalert(1)"],
    ["data:text/html;base64,PHNjcmlwdD4="],
    ["ftp://example.com/file"],
    ["mailto:someone@example.com"],
    ["vbscript:msgbox"],
    ["http://"],
    ["https://localhostx"],
    ["www."],
    ["user@www.example.com"],
    ["xhttps://example.com"],
  ])("does not link %s", (text) => {
    expect(links(text)).toEqual([]);
  });

  it("only produces http(s) hrefs", () => {
    const tricky = "javascript:alert('https://evil.com') http://ok.com <a href=x>www.fine.org</a>";
    for (const l of links(tricky)) expect(l.href).toMatch(/^https?:\/\//);
  });

  it("rejects credentials in URLs", () => {
    expect(safeHref("https://user:pass@example.com")).toBeNull();
  });

  it("trimUrl leaves balanced brackets alone", () => {
    expect(trimUrl("https://x.com/a_(b)")).toBe("https://x.com/a_(b)");
    expect(trimUrl("https://x.com/a)")).toBe("https://x.com/a");
  });
});

describe("emoji-only detection", () => {
  it.each([
    ["🔮", 1],
    ["🔮✨🙏", 3],
    [" 😀  😀 ", 2],
    ["❤️", 1],
    ["👍🏽", 1],
    ["👩‍👩‍👧", 1],
    ["🇫🇷🇪🇸", 2],
    ["1️⃣", 1],
    ["🏴󠁧󠁢󠁳󠁣󠁴󠁿", 1],
  ])("%s is emoji only (%i)", (text, n) => {
    expect(emojiOnlyCount(text)).toBe(n);
    expect(isEmojiOnly(text)).toBe(true);
  });

  it.each([["hello 👋"], ["123"], [""], ["   "], ["#"], [":)"]])("%s is not emoji only", (text) => {
    expect(isEmojiOnly(text)).toBe(false);
  });

  it("long emoji runs are not enlarged", () => {
    expect(isEmojiOnly("😀".repeat(9))).toBe(false);
  });
});

describe("insertAtCaret", () => {
  it("inserts at the caret and replaces a selection", () => {
    expect(insertAtCaret("Hello world", 5, 5, "🙂")).toEqual({ value: "Hello🙂 world", caret: 7 });
    expect(insertAtCaret("Hello world", 6, 11, "✨")).toEqual({ value: "Hello ✨", caret: 7 });
  });

  it("clamps out-of-range positions", () => {
    expect(insertAtCaret("abc", 10, 12, "!")).toEqual({ value: "abc!", caret: 4 });
  });
});
