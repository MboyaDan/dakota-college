-- M-Pesa C2B (Stage 15a, brief 9, AC-11, DG-02). Students pay the college
-- paybill with their student number as the account number; Safaricom
-- calls our confirmation URL for each payment. Every authenticated callback
-- is stored as received before anything is done with it; each M-Pesa
-- transaction is then recorded once (its code is unique), matched to a
-- student, and confirmed and receipted as a payment ("provider"). Payments
-- that match no student wait for Finance to assign them (D-170).

-- +goose Up
INSERT INTO permissions (key, module, description) VALUES
  ('finance.mpesa.setup', 'finance', 'Register the M-Pesa callback URLs with Safaricom and reprocess stored callbacks');
INSERT INTO role_permissions (role_id, permission_key)
SELECT id, 'finance.mpesa.setup' FROM roles WHERE key = 'super_admin'
ON CONFLICT DO NOTHING;

-- Raw callbacks, exactly as received. Append-only.
CREATE TABLE mpesa_callbacks (
  id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  kind         text        NOT NULL CHECK (kind IN ('confirmation', 'validation')),
  remote_ip    text        NOT NULL DEFAULT '',
  body         jsonb       NOT NULL,
  received_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX mpesa_callbacks_received_idx ON mpesa_callbacks (received_at);
CREATE TRIGGER mpesa_callbacks_append_only BEFORE UPDATE OR DELETE ON mpesa_callbacks
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

-- One row per M-Pesa transaction code. matched: paid to a student found by
-- the account number; unmatched: waiting for Finance; assigned: Finance
-- chose the student; refund: Finance will return it outside the system;
-- rejected: not for this paybill.
CREATE TABLE mpesa_transactions (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  trans_id      text        NOT NULL UNIQUE CHECK (trans_id ~ '^[A-Z0-9]{10}$'),
  trans_type    text        NOT NULL DEFAULT '',
  trans_time    timestamptz NOT NULL,
  amount_cents  bigint      NOT NULL CHECK (amount_cents > 0),
  short_code    text        NOT NULL,
  bill_ref      text        NOT NULL DEFAULT '',
  msisdn        text        NOT NULL DEFAULT '',
  payer_name    text        NOT NULL DEFAULT '',
  callback_id   bigint      NOT NULL REFERENCES mpesa_callbacks (id) ON DELETE RESTRICT,
  status        text        NOT NULL CHECK (status IN ('matched', 'unmatched', 'assigned', 'refund', 'rejected')),
  reason        text        NOT NULL DEFAULT '',
  student_id    uuid        REFERENCES students (id) ON DELETE RESTRICT,
  payment_id    uuid        UNIQUE REFERENCES payments (id) ON DELETE RESTRICT,
  handled_by    uuid        REFERENCES users (id) ON DELETE RESTRICT,
  handled_at    timestamptz,
  note          text        NOT NULL DEFAULT '' CHECK (length(note) <= 1000),
  created_at    timestamptz NOT NULL DEFAULT now(),
  CHECK ((status IN ('matched', 'assigned')) = (payment_id IS NOT NULL)),
  CHECK (status <> 'assigned' OR handled_by IS NOT NULL),
  CHECK (status <> 'refund' OR (handled_by IS NOT NULL AND length(trim(note)) > 0))
);
CREATE INDEX mpesa_transactions_status_idx ON mpesa_transactions (status, trans_time);
CREATE INDEX mpesa_transactions_time_idx ON mpesa_transactions (trans_time);

-- +goose StatementBegin
-- Only an unmatched transaction is ever decided, and what Safaricom sent never changes.
CREATE FUNCTION mpesa_transactions_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'M-Pesa transactions are never deleted' USING ERRCODE = 'restrict_violation';
  END IF;
  IF OLD.status <> 'unmatched' THEN
    RAISE EXCEPTION 'M-Pesa transaction % is %; it cannot change', OLD.trans_id, OLD.status USING ERRCODE = 'restrict_violation';
  END IF;
  IF (NEW.trans_id, NEW.trans_type, NEW.trans_time, NEW.amount_cents, NEW.short_code, NEW.bill_ref, NEW.msisdn, NEW.payer_name, NEW.callback_id, NEW.created_at)
     IS DISTINCT FROM
     (OLD.trans_id, OLD.trans_type, OLD.trans_time, OLD.amount_cents, OLD.short_code, OLD.bill_ref, OLD.msisdn, OLD.payer_name, OLD.callback_id, OLD.created_at) THEN
    RAISE EXCEPTION 'what Safaricom sent cannot change' USING ERRCODE = 'restrict_violation';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER mpesa_transactions_guard BEFORE UPDATE OR DELETE ON mpesa_transactions
  FOR EACH ROW EXECUTE FUNCTION mpesa_transactions_guard();

-- A payment confirmed by the provider flow has no member of staff who
-- recorded it.
ALTER TABLE payments ALTER COLUMN recorded_by DROP NOT NULL;
ALTER TABLE payments ADD CONSTRAINT payments_recorder_check CHECK (recorded_by IS NOT NULL OR confirmed_via = 'provider');

-- +goose Down
ALTER TABLE payments DROP CONSTRAINT payments_recorder_check;
-- Fails if provider payments exist; development data only.
ALTER TABLE payments ALTER COLUMN recorded_by SET NOT NULL;
DROP TABLE mpesa_transactions;
DROP FUNCTION mpesa_transactions_guard();
DROP TABLE mpesa_callbacks;
DELETE FROM role_permissions WHERE permission_key = 'finance.mpesa.setup';
DELETE FROM permissions WHERE key = 'finance.mpesa.setup';
