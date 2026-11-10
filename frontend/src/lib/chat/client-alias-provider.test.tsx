// @vitest-environment jsdom
import * as React from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";

import { ApiError } from "@/lib/api/errors";
import type { ClientAlias } from "@/lib/api/types";

const h = vi.hoisted(() => ({
  session: { status: "authenticated", user: { id: "adv", role: "psychic" } } as { status: string; user: { id: string; role: string } | null },
  handlers: new Map<string, (data: unknown) => void>(),
  lookup: vi.fn(),
  set: vi.fn(),
  clear: vi.fn(),
}));
vi.mock("@/lib/auth/session-provider", () => ({
  useSession: () => h.session,
  useRealtimeEvent: (type: string, handler: (data: unknown) => void) => {
    h.handlers.set(type, handler);
  },
}));
vi.mock("@/lib/api/endpoints", () => ({
  aliasApi: { lookup: (...a: unknown[]) => h.lookup(...a), set: (...a: unknown[]) => h.set(...a), clear: (...a: unknown[]) => h.clear(...a) },
}));

import {
  ALIAS_BATCH_MS,
  ALIAS_LOOKUP_MAX,
  ClientAliasProvider,
  normalizeAlias,
  useClientAliasActions,
  useClientName,
  useClientNames,
  usePrimeClientAliases,
} from "@/lib/chat/client-alias-provider";

const alias = (id: string, name: string): ClientAlias => ({ client_id: id, alias: name, updated_at: "2026-11-05T10:00:00Z" });
const emit = (type: string, data: unknown = {}) => act(() => h.handlers.get(type)?.(data));
const tick = (ms = ALIAS_BATCH_MS + 5) => act(async () => void (await vi.advanceTimersByTimeAsync(ms)));

function Name({ id, name = "Real" }: { id: string; name?: string }) {
  return <span data-testid={`n-${id}`}>{useClientName(id, name)}</span>;
}
const held: { current?: ReturnType<typeof useClientAliasActions> } = {};
const actions = () => held.current!;
function Actions() {
  const own = useClientAliasActions();
  React.useEffect(() => {
    held.current = own;
  });
  return null;
}
function mount(children: React.ReactNode) {
  return render(
    <ClientAliasProvider>
      <Actions />
      {children}
    </ClientAliasProvider>,
  );
}
const text = (id: string) => screen.getByTestId(`n-${id}`).textContent;

beforeEach(() => {
  vi.useFakeTimers();
  h.session = { status: "authenticated", user: { id: "adv", role: "psychic" } };
  h.handlers.clear();
  h.lookup.mockReset().mockResolvedValue([]);
  h.set.mockReset();
  h.clear.mockReset().mockResolvedValue(undefined);
});
afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

describe("ClientAliasProvider (§23.4)", () => {
  it("shows the real name until a nickname is known, then the nickname", async () => {
    h.lookup.mockResolvedValue([alias("c1", "Moon Paul")]);
    mount(<><Name id="c1" name="Paul" /><Name id="c2" name="Anna" /></>);
    expect(text("c1")).toBe("Paul");
    await tick();
    expect(text("c1")).toBe("Moon Paul");
    expect(text("c2")).toBe("Anna");
  });

  it("resolves unknown clients in one deduplicated batch", async () => {
    const { rerender } = mount(<><Name id="c1" /><Name id="c1" /><Name id="c2" /><Name id="c3" /></>);
    await tick();
    expect(h.lookup).toHaveBeenCalledTimes(1);
    expect(h.lookup.mock.calls[0][0].sort()).toEqual(["c1", "c2", "c3"]);
    // Known now (even "no nickname"): showing the client again asks for nothing.
    h.lookup.mockClear();
    rerender(
      <ClientAliasProvider>
        <Actions />
        <Name id="c1" />
        <Name id="c2" />
        <Name id="c4" />
      </ClientAliasProvider>,
    );
    await tick();
    expect(h.lookup).toHaveBeenCalledTimes(1);
    expect(h.lookup.mock.calls[0][0]).toEqual(["c4"]);
  });

  it("asks for at most 50 ids per request", async () => {
    const ids = Array.from({ length: ALIAS_LOOKUP_MAX + 20 }, (_, i) => `c${i}`);
    mount(<>{ids.map((id) => <Name key={id} id={id} />)}</>);
    await tick();
    await tick();
    expect(h.lookup).toHaveBeenCalledTimes(2);
    expect(h.lookup.mock.calls.map((c) => c[0].length)).toEqual([50, 20]);
    expect(new Set(h.lookup.mock.calls.flatMap((c) => c[0])).size).toBe(70);
  });

  it("tolerates a failing lookup: the real name stays, nothing throws", async () => {
    h.lookup.mockRejectedValue(new ApiError({ code: "internal_error", message: "boom", status: 500 }));
    mount(<Name id="c1" name="Paul" />);
    await tick();
    expect(h.lookup).toHaveBeenCalledTimes(1);
    expect(text("c1")).toBe("Paul");
  });

  it("is primed by rows already loaded (no request) and priming never overrides a known nickname", async () => {
    function Prime({ entries }: { entries: { clientId: string; alias?: string }[] }) {
      usePrimeClientAliases(entries);
      return null;
    }
    const first = [{ clientId: "c1", alias: "Moon Paul" }, { clientId: "c2" }];
    const { rerender } = mount(<><Prime entries={first} /><Name id="c1" name="Paul" /><Name id="c2" name="Anna" /></>);
    await tick();
    expect(text("c1")).toBe("Moon Paul");
    expect(text("c2")).toBe("Anna");
    expect(h.lookup).not.toHaveBeenCalled();
    // A stale row (an older inbox page) cannot undo a nickname the realtime event just set.
    await emit("client_alias.updated", { client_id: "c1", alias: "Sun Paul", updated_at: "x" });
    rerender(
      <ClientAliasProvider>
        <Actions />
        <Prime entries={[{ clientId: "c1", alias: "Moon Paul" }]} />
        <Name id="c1" name="Paul" />
        <Name id="c2" name="Anna" />
      </ClientAliasProvider>,
    );
    expect(text("c1")).toBe("Sun Paul");
  });

  it("applies client_alias.updated, and a null alias goes back to the real name", async () => {
    h.lookup.mockResolvedValue([alias("c1", "Moon Paul")]);
    mount(<Name id="c1" name="Paul" />);
    await tick();
    await emit("client_alias.updated", { client_id: "c1", alias: "Sun Paul", updated_at: "x" });
    expect(text("c1")).toBe("Sun Paul");
    await emit("client_alias.updated", { client_id: "c1", alias: null, updated_at: "y" });
    expect(text("c1")).toBe("Paul");
  });

  it("a lookup that started earlier cannot overwrite a newer event", async () => {
    let release!: (items: ClientAlias[]) => void;
    h.lookup.mockReturnValue(new Promise((r) => (release = r)));
    mount(<Name id="c1" name="Paul" />);
    await tick();
    await emit("client_alias.updated", { client_id: "c1", alias: "Newer", updated_at: "x" });
    await act(async () => release([alias("c1", "Older")]));
    expect(text("c1")).toBe("Newer");
  });

  it("set is optimistic and confirmed by the answer; the API gets the trimmed, collapsed value", async () => {
    let answer!: (a: ClientAlias) => void;
    h.set.mockReturnValue(new Promise((r) => (answer = r)));
    mount(<Name id="c1" name="Paul" />);
    await tick();
    let done!: Promise<void>;
    await act(async () => {
      done = actions().setAlias("conv-1", "c1", "  Moon   Paul ");
    });
    expect(text("c1")).toBe("Moon Paul"); // before the API answered
    expect(h.set).toHaveBeenCalledWith("conv-1", "Moon Paul");
    await act(async () => {
      answer(alias("c1", "Moon Paul"));
      await done;
    });
    expect(text("c1")).toBe("Moon Paul");
  });

  it("rolls back and rejects with the API's error when saving fails", async () => {
    h.lookup.mockResolvedValue([alias("c1", "Old")]);
    h.set.mockRejectedValue(new ApiError({ code: "validation_failed", message: "bad", status: 422, details: { fields: { alias: "No control characters." } } }));
    mount(<Name id="c1" name="Paul" />);
    await tick();
    expect(text("c1")).toBe("Old");
    let error: unknown;
    await act(async () => {
      error = await actions().setAlias("conv-1", "c1", "New").catch((e) => e);
    });
    expect(error).toBeInstanceOf(ApiError);
    expect(text("c1")).toBe("Old");
  });

  it("an empty value clears the nickname (DELETE) and shows the real name; a failed clear is rolled back", async () => {
    h.lookup.mockResolvedValue([alias("c1", "Moon Paul")]);
    mount(<Name id="c1" name="Paul" />);
    await tick();
    await act(async () => void (await actions().setAlias("conv-1", "c1", "   ")));
    expect(h.clear).toHaveBeenCalledWith("conv-1");
    expect(h.set).not.toHaveBeenCalled();
    expect(text("c1")).toBe("Paul");

    h.lookup.mockResolvedValue([alias("c2", "Keep me")]);
    h.clear.mockRejectedValue(new ApiError({ code: "internal_error", message: "x", status: 500 }));
    cleanup();
    mount(<Name id="c2" name="Anna" />);
    await tick();
    await act(async () => void (await actions().clearAlias("conv-2", "c2").catch(() => undefined)));
    expect(text("c2")).toBe("Keep me");
  });

  it("forgets the cache when events may have been lost (resync) and asks again", async () => {
    h.lookup.mockResolvedValueOnce([alias("c1", "Moon Paul")]).mockResolvedValueOnce([alias("c1", "Changed")]);
    mount(<Name id="c1" name="Paul" />);
    await tick();
    expect(text("c1")).toBe("Moon Paul");
    await emit("resync_required");
    await tick();
    expect(h.lookup).toHaveBeenCalledTimes(2);
    expect(text("c1")).toBe("Changed");
  });

  it("useClientNames gives names for lists and says when an answer is final", async () => {
    h.lookup.mockResolvedValue([alias("c1", "Moon Paul")]);
    function List() {
      const { nameFor, isKnown } = useClientNames(["c1", "c2"]);
      return <p data-testid="list">{`${nameFor("c1", "Paul")}|${nameFor("c2", "Anna")}|${isKnown("c1")}`}</p>;
    }
    mount(<List />);
    expect(screen.getByTestId("list").textContent).toBe("Paul|Anna|false");
    await tick();
    expect(screen.getByTestId("list").textContent).toBe("Moon Paul|Anna|true");
  });

  it("resolve() answers a one-off question (a toast) from the cache or a lookup, and gives up after a while", async () => {
    h.lookup.mockResolvedValue([alias("c1", "Moon Paul")]);
    mount(null);
    const first = actions().resolve("c1");
    await tick();
    await expect(first).resolves.toBe("Moon Paul");
    await expect(actions().resolve("c1")).resolves.toBe("Moon Paul");
    h.lookup.mockReturnValue(new Promise(() => undefined));
    const stuck = actions().resolve("c9");
    await tick(2000);
    await expect(stuck).resolves.toBeUndefined();
  });
});

describe("for clients and signed-out visitors", () => {
  it.each([
    ["a client", { status: "authenticated", user: { id: "cl", role: "client" } }],
    ["a visitor", { status: "anonymous", user: null }],
  ])("%s: real names, no request, no editing", async (_label, session) => {
    h.session = session;
    mount(<Name id="c1" name="Paul" />);
    await tick(500);
    expect(text("c1")).toBe("Paul");
    expect(h.lookup).not.toHaveBeenCalled();
    expect(actions().enabled).toBe(false);
    await emit("client_alias.updated", { client_id: "c1", alias: "Leak", updated_at: "x" });
    expect(text("c1")).toBe("Paul");
    await expect(actions().resolve("c1")).resolves.toBeUndefined();
    expect(h.lookup).not.toHaveBeenCalled();
  });

  it("works without any provider (isolated components)", () => {
    render(<Name id="c1" name="Paul" />);
    expect(text("c1")).toBe("Paul");
  });
});

describe("normalizeAlias", () => {
  it("trims and collapses inner whitespace like the API", () => {
    expect(normalizeAlias("  Moon \t  Paul ")).toBe("Moon Paul");
    expect(normalizeAlias("   ")).toBe("");
  });
});
