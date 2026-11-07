-- 0019_chat_unread: cheap per-user unread totals for the header chat badge.
--
-- GET /chat/unread-count sums client_unread (or psychic_unread) over a user's
-- conversations. The partial indexes hold only conversations that actually
-- have unread messages and INCLUDE the counter, so the sum is an index-only
-- scan whose size follows the number of unread threads, not the number of
-- conversations (an advisor can have tens of thousands).
CREATE INDEX IF NOT EXISTS conversations_client_unread_idx ON conversations (client_id) INCLUDE (client_unread) WHERE client_unread > 0;
CREATE INDEX IF NOT EXISTS conversations_psychic_unread_idx ON conversations (psychic_id) INCLUDE (psychic_unread) WHERE psychic_unread > 0;
