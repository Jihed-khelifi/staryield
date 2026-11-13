"use client";

import { useI18n } from "@/i18n/i18n-provider";
import type { Transaction } from "@/lib/api/billing";
import { REFUND_STATUS_COPY, refundRowState } from "@/lib/billing/refund";

import { formatDay } from "../format";

/**
 * Under a credit purchase in the billing history (§24.4): "Request a refund"
 * when one can be sent, or the request's status, its reference and "Withdraw"
 * while it is open. Renders nothing for other payments.
 */
export function RefundStatusLine({
  transaction,
  timeZone,
  onRequest,
  onWithdraw,
}: {
  transaction: Transaction;
  timeZone?: string;
  onRequest: (transaction: Transaction) => void;
  onWithdraw: (request: { id: string; reference: string }) => void;
}) {
  const { text, locale } = useI18n();
  const state = refundRowState(transaction.refund);
  if (state.kind === "none") return null;
  const day = formatDay(transaction.created_at, locale, timeZone);
  const requestButton = (
    <button type="button" className="settings-link-button settings-refund-action" onClick={() => onRequest(transaction)} data-testid="refund-request">
      {text("Request a refund")}
      <span className="sr-only"> {text("for the payment on {value0}", { value0: day })}</span>
    </button>
  );
  if (state.kind === "request") return <div className="settings-transaction-refund">{requestButton}</div>;
  return (
    <div className="settings-transaction-refund" data-testid="refund-status">
      <span className="settings-status" data-status={`refund_${state.status}`}>
        {text(REFUND_STATUS_COPY[state.status])}
      </span>
      <span className="settings-row-hint" data-testid="refund-reference-label">
        {text("Reference {value0}", { value0: state.reference })}
      </span>
      {state.withdrawable ? (
        <button type="button" className="settings-link-button settings-refund-action" onClick={() => onWithdraw({ id: state.id, reference: state.reference })} data-testid="refund-withdraw">
          {text("Withdraw")}
          <span className="sr-only"> {text("refund request {value0}", { value0: state.reference })}</span>
        </button>
      ) : null}
      {state.canRequestAgain ? requestButton : null}
    </div>
  );
}
