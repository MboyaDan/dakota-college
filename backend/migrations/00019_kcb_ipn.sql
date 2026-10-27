-- KCB Buni Instant Payment Notifications (Stage 15b, brief 9, AC-11). KCB
-- POSTs a notification after crediting the college account; the payer's
-- reference is the student number. Every accepted notification is stored as
-- received (with its Signature header) before it is processed; each KCB
-- transaction reference is recorded once.
--
-- Until KCB supplies the key and method to verify the Signature header, a
-- notification is not proof of payment (D-175): a match becomes a pending
-- bank payment that Finance confirms against the KCB statement. verified
-- records whether the signature was checked, for when that becomes possible.

-- +goose Up
CREATE TABLE kcb_notifications (
  id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  remote_ip    text        NOT NULL DEFAULT '',
  signature    text        NOT NULL DEFAULT '' CHECK (length(signature) <= 4096),
  body         jsonb       NOT NULL,
  received_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX kcb_notifications_received_idx ON kcb_notifications (received_at);
CREATE TRIGGER kcb_notifications_append_only BEFORE UPDATE OR DELETE ON kcb_notifications
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

-- matched: a pending (or, once verified, confirmed) payment exists for the
-- student; unmatched: waiting for Finance; assigned: Finance chose the
-- student; refund: to be returned outside the system; rejected: not for
-- the college account, or not in KES.
CREATE TABLE kcb_transactions (
  id                     uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  transaction_reference  text        NOT NULL UNIQUE CHECK (transaction_reference ~ '^[A-Za-z0-9_-]{1,64}$'),
  request_id             text        NOT NULL DEFAULT '',
  channel_code           text        NOT NULL DEFAULT '',
  sent_at                timestamptz NOT NULL,
  sent_at_raw            text        NOT NULL DEFAULT '',
  amount_cents           bigint      NOT NULL CHECK (amount_cents > 0),
  currency               text        NOT NULL,
  customer_reference     text        NOT NULL DEFAULT '',
  customer_name          text        NOT NULL DEFAULT '',
  customer_mobile        text        NOT NULL DEFAULT '',
  narration              text        NOT NULL DEFAULT '',
  credit_account         text        NOT NULL DEFAULT '',
  org_short_code         text        NOT NULL DEFAULT '',
  till_number            text        NOT NULL DEFAULT '',
  notification_id        bigint      NOT NULL REFERENCES kcb_notifications (id) ON DELETE RESTRICT,
  verified               boolean     NOT NULL DEFAULT false,
  status                 text        NOT NULL CHECK (status IN ('matched', 'unmatched', 'assigned', 'refund', 'rejected')),
  reason                 text        NOT NULL DEFAULT '',
  student_id             uuid        REFERENCES students (id) ON DELETE RESTRICT,
  payment_id             uuid        UNIQUE REFERENCES payments (id) ON DELETE RESTRICT,
  handled_by             uuid        REFERENCES users (id) ON DELETE RESTRICT,
  handled_at             timestamptz,
  note                   text        NOT NULL DEFAULT '' CHECK (length(note) <= 1000),
  created_at             timestamptz NOT NULL DEFAULT now(),
  CHECK ((status IN ('matched', 'assigned')) = (payment_id IS NOT NULL)),
  CHECK (status <> 'assigned' OR handled_by IS NOT NULL),
  CHECK (status <> 'refund' OR (handled_by IS NOT NULL AND length(trim(note)) > 0))
);
CREATE INDEX kcb_transactions_status_idx ON kcb_transactions (status, sent_at);
CREATE INDEX kcb_transactions_time_idx ON kcb_transactions (sent_at);

-- +goose StatementBegin
-- Only an unmatched transaction is ever decided, and what KCB sent never changes.
CREATE FUNCTION kcb_transactions_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'KCB transactions are never deleted' USING ERRCODE = 'restrict_violation';
  END IF;
  IF OLD.status <> 'unmatched' THEN
    RAISE EXCEPTION 'KCB transaction % is %; it cannot change', OLD.transaction_reference, OLD.status USING ERRCODE = 'restrict_violation';
  END IF;
  IF (NEW.transaction_reference, NEW.request_id, NEW.channel_code, NEW.sent_at, NEW.sent_at_raw, NEW.amount_cents, NEW.currency,
      NEW.customer_reference, NEW.customer_name, NEW.customer_mobile, NEW.narration, NEW.credit_account, NEW.org_short_code,
      NEW.till_number, NEW.notification_id, NEW.verified, NEW.created_at)
     IS DISTINCT FROM
     (OLD.transaction_reference, OLD.request_id, OLD.channel_code, OLD.sent_at, OLD.sent_at_raw, OLD.amount_cents, OLD.currency,
      OLD.customer_reference, OLD.customer_name, OLD.customer_mobile, OLD.narration, OLD.credit_account, OLD.org_short_code,
      OLD.till_number, OLD.notification_id, OLD.verified, OLD.created_at) THEN
    RAISE EXCEPTION 'what KCB sent cannot change' USING ERRCODE = 'restrict_violation';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER kcb_transactions_guard BEFORE UPDATE OR DELETE ON kcb_transactions
  FOR EACH ROW EXECUTE FUNCTION kcb_transactions_guard();

-- Where a payment came from. Staff payments have a recorder; payments made
-- from a provider's notification (M-Pesa, KCB) may not.
ALTER TABLE payments ADD COLUMN source text NOT NULL DEFAULT 'staff' CHECK (source IN ('staff', 'mpesa', 'kcb'));
ALTER TABLE payments DISABLE TRIGGER payments_guard;
UPDATE payments SET source = 'mpesa' WHERE id IN (SELECT payment_id FROM mpesa_transactions WHERE payment_id IS NOT NULL);
ALTER TABLE payments ENABLE TRIGGER payments_guard;
ALTER TABLE payments DROP CONSTRAINT payments_recorder_check;
ALTER TABLE payments ADD CONSTRAINT payments_recorder_check CHECK (recorded_by IS NOT NULL OR source <> 'staff');

-- +goose StatementBegin
-- As before (00017), and the source never changes either.
CREATE OR REPLACE FUNCTION payments_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'payments are never deleted' USING ERRCODE = 'restrict_violation';
  END IF;
  IF OLD.status <> 'pending' THEN
    RAISE EXCEPTION 'payment % is %; it cannot change', OLD.id, OLD.status USING ERRCODE = 'restrict_violation';
  END IF;
  IF (NEW.id, NEW.student_id, NEW.method, NEW.amount_cents, NEW.paid_on, NEW.reference, NEW.payer_name, NEW.note, NEW.recorded_by, NEW.recorded_at, NEW.source)
     IS DISTINCT FROM
     (OLD.id, OLD.student_id, OLD.method, OLD.amount_cents, OLD.paid_on, OLD.reference, OLD.payer_name, OLD.note, OLD.recorded_by, OLD.recorded_at, OLD.source) THEN
    RAISE EXCEPTION 'what was received cannot change; reject the payment and record it again' USING ERRCODE = 'restrict_violation';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE kcb_transactions;
DROP FUNCTION kcb_transactions_guard();
DROP TABLE kcb_notifications;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION payments_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'payments are never deleted' USING ERRCODE = 'restrict_violation';
  END IF;
  IF OLD.status <> 'pending' THEN
    RAISE EXCEPTION 'payment % is %; it cannot change', OLD.id, OLD.status USING ERRCODE = 'restrict_violation';
  END IF;
  IF (NEW.id, NEW.student_id, NEW.method, NEW.amount_cents, NEW.paid_on, NEW.reference, NEW.payer_name, NEW.note, NEW.recorded_by, NEW.recorded_at)
     IS DISTINCT FROM
     (OLD.id, OLD.student_id, OLD.method, OLD.amount_cents, OLD.paid_on, OLD.reference, OLD.payer_name, OLD.note, OLD.recorded_by, OLD.recorded_at) THEN
    RAISE EXCEPTION 'what was received cannot change; reject the payment and record it again' USING ERRCODE = 'restrict_violation';
  END IF;
  RETURN NEW;
END;
$$;
-- +goose StatementEnd
ALTER TABLE payments DROP CONSTRAINT payments_recorder_check;
-- Fails if KCB notification payments without a recorder exist; development data only.
ALTER TABLE payments ADD CONSTRAINT payments_recorder_check CHECK (recorded_by IS NOT NULL OR confirmed_via = 'provider');
ALTER TABLE payments DROP COLUMN source;
