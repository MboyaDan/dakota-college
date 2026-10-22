-- name: NextCounterValue :one
-- Gapless sequence per (name, period). Call inside the transaction that uses
-- the number: the row lock makes concurrent callers wait, and a rollback
-- returns the number, so no gaps appear (A-06).
INSERT INTO document_counters (name, period, last_value)
VALUES (@name, @period, 1)
ON CONFLICT (name, period) DO UPDATE SET last_value = document_counters.last_value + 1
RETURNING last_value;
