import { describe, expect, it } from "vitest";

import { isRefundAlias, REFUND_POLICY_ALIASES } from "@/lib/legal/refund-aliases";

describe("refund policy aliases (revisions 5.1)", () => {
  it("covers the four documented paths", () => {
    expect([...REFUND_POLICY_ALIASES].sort()).toEqual(["refund", "refunds", "refunds-policy", "reimburse"]);
  });

  it("matches the aliases, ignoring case and a trailing slash, and nothing else", () => {
    for (const alias of REFUND_POLICY_ALIASES) {
      expect(isRefundAlias(alias)).toBe(true);
      expect(isRefundAlias(`${alias}/`)).toBe(true);
      expect(isRefundAlias(alias.toUpperCase())).toBe(true);
    }
    for (const other of ["refund-policy", "refunded", "pricing", "", "refunds/extra"]) expect(isRefundAlias(other)).toBe(false);
  });
});
