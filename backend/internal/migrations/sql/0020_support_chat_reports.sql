-- 0020_support_chat_reports: conversation reports (revisions 3.1, section 16.4).
--
-- A client reports a problem with a chat conversation from the chatroom. The
-- report is an ordinary support ticket (same thread, replies, e-mails, private
-- link, auto-close, retention and erasure) of kind 'chat_report', linked to the
-- conversation, one of its sessions and the advisor's user. The advisor is
-- never told: nothing here is readable through any advisor endpoint.

ALTER TABLE support_tickets
    ADD COLUMN kind             text   NOT NULL DEFAULT 'contact',
    ADD COLUMN conversation_id  uuid   REFERENCES conversations (id) ON DELETE SET NULL,
    ADD COLUMN session_id       uuid   REFERENCES chat_sessions (id) ON DELETE SET NULL,
    ADD COLUMN reported_user_id uuid   REFERENCES users (id) ON DELETE SET NULL,
    ADD COLUMN report_reason    text,
    -- newest message sequence the reporter had seen (clamped to the conversation's last_seq).
    ADD COLUMN reported_seq     bigint NOT NULL DEFAULT 0;

ALTER TABLE support_tickets
    ADD CONSTRAINT support_tickets_kind_check CHECK (kind IN ('contact', 'chat_report')),
    ADD CONSTRAINT support_tickets_report_reason_check CHECK (
        (kind = 'contact' AND report_reason IS NULL)
        OR (kind = 'chat_report' AND report_reason IS NOT NULL AND report_reason IN ('advisor_unresponsive', 'inappropriate', 'billing', 'technical', 'other'))),
    ADD CONSTRAINT support_tickets_reported_seq_check CHECK (reported_seq >= 0);

-- A report's details are optional (0-2000 characters), so the request length
-- bound of contact tickets (10-5000) only applies to them.
ALTER TABLE support_tickets DROP CONSTRAINT support_tickets_message_chars_check;
ALTER TABLE support_tickets ADD CONSTRAINT support_tickets_message_chars_check CHECK (
    (kind = 'contact' AND message_chars BETWEEN 10 AND 5000)
    OR (kind = 'chat_report' AND message_chars BETWEEN 0 AND 2000));

-- One open or pending report per reporter and conversation. It also serves the
-- open-report lookup (user_id, conversation_id) and the console's reports_open
-- counter (index-only count over this partial index).
CREATE UNIQUE INDEX support_tickets_one_open_report_idx ON support_tickets (user_id, conversation_id)
    WHERE kind = 'chat_report' AND status IN ('open', 'pending') AND deleted_at IS NULL;

-- The console's kind=chat_report filter, one partial index per sort (the same
-- shapes as the contact queue: newest/oldest, waiting open, waiting rest).
CREATE INDEX support_tickets_report_created_idx ON support_tickets (created_at DESC, id DESC)
    WHERE kind = 'chat_report' AND deleted_at IS NULL;
CREATE INDEX support_tickets_report_status_idx ON support_tickets (status, created_at DESC, id DESC)
    WHERE kind = 'chat_report' AND deleted_at IS NULL;
CREATE INDEX support_tickets_report_waiting_idx ON support_tickets (last_customer_message_at, id)
    WHERE kind = 'chat_report' AND status = 'open' AND deleted_at IS NULL;
CREATE INDEX support_tickets_report_waiting_rest_idx ON support_tickets (last_customer_message_at, id)
    WHERE kind = 'chat_report' AND status <> 'open' AND deleted_at IS NULL;

-- Foreign keys with ON DELETE SET NULL: without these, erasing an advisor, a
-- conversation or a session would scan the whole ticket table.
CREATE INDEX support_tickets_conversation_idx ON support_tickets (conversation_id) WHERE conversation_id IS NOT NULL;
CREATE INDEX support_tickets_session_idx ON support_tickets (session_id) WHERE session_id IS NOT NULL;
CREATE INDEX support_tickets_reported_user_idx ON support_tickets (reported_user_id) WHERE reported_user_id IS NOT NULL;

COMMENT ON COLUMN support_tickets.reported_user_id IS
    'The advisor''s user a chat report is about. Never exposed to advisors: only the reporter and the console read it.';
