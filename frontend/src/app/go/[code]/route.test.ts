import { describe, expect, it, vi } from "vitest";

vi.mock("@/lib/api/config", () => ({ API_URL: "https://api.staryield.net" }));

import { GET } from "@/app/go/[code]/route";
import { shortLinkTarget } from "@/lib/short-links";

const call = (code: string) => GET(new Request(`https://staryield.net/go/${encodeURIComponent(code)}`), { params: Promise.resolve({ code }) });

describe("/go/[code]", () => {
  it("redirects valid codes to the API click tracker", async () => {
    const res = await call("aB3dE9x");
    expect(res.status).toBe(302);
    expect(res.headers.get("Location")).toBe("https://api.staryield.net/l/aB3dE9x");
    expect(res.headers.get("Cache-Control")).toBe("no-store");
  });

  it.each(["spring_sale-2026", "abc", "A".repeat(32)])("accepts %s", async (code) => {
    expect((await call(code)).headers.get("Location")).toBe(`https://api.staryield.net/l/${code}`);
  });

  it.each(["ab", "A".repeat(33), "bad code", "../admin", "x?y=1", "évé", "<script>", "a/b"])("sends malformed code %s home", async (code) => {
    const res = await call(code);
    expect(res.status).toBe(302);
    expect(res.headers.get("Location")).toBe("https://staryield.net/");
  });

  it("builds targets from the configured API origin", () => {
    expect(shortLinkTarget("abc123", "http://localhost:3000/go/abc123", "http://localhost:8080")).toBe("http://localhost:8080/l/abc123");
    expect(shortLinkTarget("no", "http://localhost:3000/go/no", "http://localhost:8080")).toBe("http://localhost:3000/");
  });
});
