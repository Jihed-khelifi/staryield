-- 0015_perf_indexes: indexes found by the large-dataset query-plan checks
-- (tests/load, TestExplainHotQueries).
--
-- The public advisor catalogue joins every listed advisor to their account
-- to keep only active ones. Without this index the planner hash-joins the
-- whole users table (a parallel sequential scan: ~45 ms at 200k users,
-- growing with every new client). The partial index holds only active
-- advisor accounts, a few thousand rows.
CREATE INDEX IF NOT EXISTS users_active_psychics_idx ON users (id) WHERE role = 'psychic' AND status = 'active';
