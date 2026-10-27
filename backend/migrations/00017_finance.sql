-- Finance core (Stage 14a, brief 9 and 10, AC-10, AC-11). Finance keeps fee
-- structures per programme and intake (D-157); invoices copy their lines,
-- so a later change to a structure never changes a bill. Money is integer
-- cents of KES (NN-03). Nothing stores a balance: it is always the sum of
-- issued invoice lines and charges, less confirmed payments and credits.
-- Issued invoices, decided payments, receipts and adjustments never change
-- (NN-05); a correction is a compensating adjustment.

-- +goose Up
INSERT INTO permissions (key, module, description) VALUES
  ('finance.payment.record', 'finance', 'Record payments received; bank, M-Pesa and sponsor payments then wait for confirmation');

INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, 'finance.payment.record' FROM roles r WHERE r.key IN ('super_admin', 'finance_officer')
ON CONFLICT DO NOTHING;

-- What a programme charges, per intake or for every intake (intake_id
-- NULL). A template for invoices: editing it changes only invoices made
-- afterwards.
CREATE TABLE fee_structures (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  programme_id  uuid        NOT NULL REFERENCES programmes (id) ON DELETE RESTRICT,
  intake_id     uuid        REFERENCES programme_intakes (id) ON DELETE RESTRICT,
  note          text        NOT NULL DEFAULT '' CHECK (length(note) <= 1000),
  is_sample     boolean     NOT NULL DEFAULT false,
  updated_by    uuid        REFERENCES users (id) ON DELETE RESTRICT,
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  UNIQUE NULLS NOT DISTINCT (programme_id, intake_id)
);
CREATE TRIGGER fee_structures_updated_at BEFORE UPDATE ON fee_structures FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- charge: once (first invoice, at enrolment) or per_term (every invoice).
CREATE TABLE fee_structure_lines (
  id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  structure_id  uuid        NOT NULL REFERENCES fee_structures (id) ON DELETE CASCADE,
  label         text        NOT NULL CHECK (length(trim(label)) BETWEEN 1 AND 120),
  category      text        NOT NULL CHECK (category IN ('tuition', 'registration', 'assessment', 'caution', 'other')),
  charge        text        NOT NULL CHECK (charge IN ('once', 'per_term')),
  amount_cents  bigint      NOT NULL CHECK (amount_cents BETWEEN 1 AND 100000000000),
  sort_order    integer     NOT NULL DEFAULT 0
);
CREATE INDEX fee_structure_lines_structure_idx ON fee_structure_lines (structure_id, sort_order);

-- How an invoice is split: a share of the total, due some days after issue.
CREATE TABLE fee_structure_instalments (
  structure_id  uuid        NOT NULL REFERENCES fee_structures (id) ON DELETE CASCADE,
  seq           smallint    NOT NULL CHECK (seq BETWEEN 1 AND 12),
  percent       smallint    NOT NULL CHECK (percent BETWEEN 1 AND 100),
  due_days      smallint    NOT NULL CHECK (due_days BETWEEN 0 AND 730),
  PRIMARY KEY (structure_id, seq)
);

-- Invoices (table from Stage 10a). Void is only for a draft that is not
-- needed; an issued invoice is corrected by an adjustment.
ALTER TABLE invoices DROP CONSTRAINT invoices_check;
ALTER TABLE invoices
  ADD COLUMN kind       text        NOT NULL DEFAULT 'enrolment' CHECK (kind IN ('enrolment', 'term', 'other')),
  ADD COLUMN term_id    uuid        REFERENCES academic_terms (id) ON DELETE RESTRICT,
  ADD COLUMN issued_at  timestamptz,
  ADD COLUMN issued_by  uuid        REFERENCES users (id) ON DELETE RESTRICT,
  ADD COLUMN void_reason text       NOT NULL DEFAULT '' CHECK (length(void_reason) <= 500),
  ADD CONSTRAINT invoices_note_length CHECK (length(note) <= 1000),
  ADD CONSTRAINT invoices_issued_check CHECK ((status = 'issued') = (number IS NOT NULL AND issued_at IS NOT NULL AND issued_by IS NOT NULL)),
  ADD CONSTRAINT invoices_term_check CHECK ((kind = 'term') = (term_id IS NOT NULL)),
  ADD CONSTRAINT invoices_void_check CHECK ((status = 'void') = (length(trim(void_reason)) > 0));
-- One enrolment invoice and one invoice per term for each student, so a
-- repeated billing run skips students already billed.
CREATE UNIQUE INDEX invoices_one_enrolment_idx ON invoices (student_id) WHERE kind = 'enrolment' AND status <> 'void';
CREATE UNIQUE INDEX invoices_one_per_term_idx ON invoices (student_id, term_id) WHERE kind = 'term' AND status <> 'void';
CREATE INDEX invoices_status_idx ON invoices (status, created_at);

CREATE TABLE invoice_lines (
  id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  invoice_id    uuid        NOT NULL REFERENCES invoices (id) ON DELETE RESTRICT,
  label         text        NOT NULL CHECK (length(trim(label)) BETWEEN 1 AND 120),
  category      text        NOT NULL CHECK (category IN ('tuition', 'registration', 'assessment', 'caution', 'other')),
  amount_cents  bigint      NOT NULL CHECK (amount_cents BETWEEN 1 AND 100000000000),
  sort_order    integer     NOT NULL DEFAULT 0
);
CREATE INDEX invoice_lines_invoice_idx ON invoice_lines (invoice_id, sort_order);

-- Amounts and due dates are fixed when the invoice is issued.
CREATE TABLE invoice_instalments (
  invoice_id    uuid        NOT NULL REFERENCES invoices (id) ON DELETE RESTRICT,
  seq           smallint    NOT NULL CHECK (seq BETWEEN 1 AND 12),
  percent       smallint    NOT NULL CHECK (percent BETWEEN 1 AND 100),
  due_days      smallint    NOT NULL CHECK (due_days BETWEEN 0 AND 730),
  amount_cents  bigint      CHECK (amount_cents >= 0),
  due_on        date,
  PRIMARY KEY (invoice_id, seq),
  CHECK ((amount_cents IS NULL) = (due_on IS NULL))
);
CREATE INDEX invoice_instalments_due_idx ON invoice_instalments (due_on);

-- +goose StatementBegin
-- An invoice changes only while it is a draft, and is never deleted.
CREATE FUNCTION invoices_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'DELETE' THEN
    RAISE EXCEPTION 'invoices are never deleted; void a draft instead' USING ERRCODE = 'restrict_violation';
  END IF;
  IF OLD.status <> 'draft' THEN
    RAISE EXCEPTION 'invoice % is %; it cannot change', OLD.id, OLD.status USING ERRCODE = 'restrict_violation';
  END IF;
  RETURN NEW;
END;
$$;

-- Lines and instalments change only while their invoice is a draft.
CREATE FUNCTION invoice_parts_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
  st text;
BEGIN
  SELECT status INTO st FROM invoices WHERE id = COALESCE(NEW.invoice_id, OLD.invoice_id);
  IF st IS DISTINCT FROM 'draft' THEN
    RAISE EXCEPTION 'invoice is %; its lines cannot change', st USING ERRCODE = 'restrict_violation';
  END IF;
  IF TG_OP = 'UPDATE' AND NEW.invoice_id <> OLD.invoice_id THEN
    RAISE EXCEPTION 'a line cannot move to another invoice' USING ERRCODE = 'restrict_violation';
  END IF;
  RETURN COALESCE(NEW, OLD);
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER invoices_guard BEFORE UPDATE OR DELETE ON invoices FOR EACH ROW EXECUTE FUNCTION invoices_guard();
CREATE TRIGGER invoice_lines_guard BEFORE INSERT OR UPDATE OR DELETE ON invoice_lines FOR EACH ROW EXECUTE FUNCTION invoice_parts_guard();
CREATE TRIGGER invoice_instalments_guard BEFORE INSERT OR UPDATE OR DELETE ON invoice_instalments FOR EACH ROW EXECUTE FUNCTION invoice_parts_guard();

-- Money received. Only a confirmed payment counts towards the balance
-- (AC-11): confirmed by a finance officer (cash at the office at once, the
-- rest against the bank or M-Pesa statement), or, from Stage 15, by the
-- verified provider flow. A typed reference alone is never proof.
CREATE TABLE payments (
  id             uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  student_id     uuid        NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
  method         text        NOT NULL CHECK (method IN ('cash', 'bank', 'mpesa', 'sponsor')),
  amount_cents   bigint      NOT NULL CHECK (amount_cents BETWEEN 1 AND 100000000000),
  paid_on        date        NOT NULL,
  -- M-Pesa transaction code, bank slip or transfer reference.
  reference      text        NOT NULL DEFAULT '' CHECK (length(reference) <= 60),
  payer_name     text        NOT NULL DEFAULT '' CHECK (length(payer_name) <= 120),
  note           text        NOT NULL DEFAULT '' CHECK (length(note) <= 1000),
  status         text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'rejected')),
  confirmed_via  text        CHECK (confirmed_via IN ('finance', 'provider')),
  recorded_by    uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  recorded_at    timestamptz NOT NULL DEFAULT now(),
  decided_by     uuid        REFERENCES users (id) ON DELETE RESTRICT,
  decided_at     timestamptz,
  decision_note  text        NOT NULL DEFAULT '' CHECK (length(decision_note) <= 1000),
  CHECK ((status = 'pending') = (decided_at IS NULL)),
  CHECK ((status = 'confirmed') = (confirmed_via IS NOT NULL)),
  CHECK (confirmed_via IS DISTINCT FROM 'finance' OR decided_by IS NOT NULL),
  CHECK (status <> 'rejected' OR (decided_by IS NOT NULL AND length(trim(decision_note)) > 0)),
  CHECK (method <> 'mpesa' OR reference ~ '^[A-Z0-9]{10}$'),
  CHECK (method <> 'bank' OR length(trim(reference)) > 0),
  CHECK (method <> 'sponsor' OR length(trim(payer_name)) > 0)
);
-- An M-Pesa code pays once.
CREATE UNIQUE INDEX payments_mpesa_reference_idx ON payments (reference) WHERE method = 'mpesa' AND status <> 'rejected';
CREATE INDEX payments_student_idx ON payments (student_id, paid_on);
CREATE INDEX payments_status_idx ON payments (status, recorded_at);

-- +goose StatementBegin
-- A payment is decided once, and what was received never changes.
CREATE FUNCTION payments_guard() RETURNS trigger LANGUAGE plpgsql AS $$
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

CREATE TRIGGER payments_guard BEFORE UPDATE OR DELETE ON payments FOR EACH ROW EXECUTE FUNCTION payments_guard();

-- One receipt per confirmed payment, numbered DTC/RCT/YYYY/NNNNN from a
-- gapless counter (A-06). issued_by is empty when the provider flow
-- confirmed the payment (Stage 15).
CREATE TABLE receipts (
  id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  number      text        NOT NULL UNIQUE,
  payment_id  uuid        NOT NULL UNIQUE REFERENCES payments (id) ON DELETE RESTRICT,
  issued_at   timestamptz NOT NULL DEFAULT now(),
  issued_by   uuid        REFERENCES users (id) ON DELETE RESTRICT
);
CREATE TRIGGER receipts_append_only BEFORE UPDATE OR DELETE ON receipts
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

-- Compensating entries (brief 9): a charge raises the balance, a credit
-- lowers it. They may point at the invoice or payment they correct.
CREATE TABLE adjustments (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  student_id    uuid        NOT NULL REFERENCES students (id) ON DELETE RESTRICT,
  kind          text        NOT NULL CHECK (kind IN ('charge', 'credit')),
  amount_cents  bigint      NOT NULL CHECK (amount_cents BETWEEN 1 AND 100000000000),
  reason        text        NOT NULL CHECK (length(trim(reason)) BETWEEN 5 AND 1000),
  invoice_id    uuid        REFERENCES invoices (id) ON DELETE RESTRICT,
  payment_id    uuid        REFERENCES payments (id) ON DELETE RESTRICT,
  created_by    uuid        NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
  created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX adjustments_student_idx ON adjustments (student_id, created_at);
CREATE TRIGGER adjustments_append_only BEFORE UPDATE OR DELETE ON adjustments
  FOR EACH ROW EXECUTE FUNCTION forbid_update_delete();

-- How students pay, shown on their Fees page. Empty hides it: the college
-- supplies the paybill, till or bank details (DQ-09), never the build.
INSERT INTO site_settings (key, value, description) VALUES
  ('finance.payment_instructions', '', 'How students pay fees (paybill, till, bank account, office hours). Shown on the student Fees page; empty hides it')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM site_settings WHERE key = 'finance.payment_instructions';
DROP TABLE adjustments;
DROP TABLE receipts;
DROP TRIGGER payments_guard ON payments;
DROP TABLE payments;
DROP FUNCTION payments_guard();
DROP TABLE invoice_instalments;
DROP TABLE invoice_lines;
DROP FUNCTION invoice_parts_guard();
DROP TRIGGER invoices_guard ON invoices;
DROP FUNCTION invoices_guard();
DROP INDEX invoices_status_idx;
DROP INDEX invoices_one_per_term_idx;
DROP INDEX invoices_one_enrolment_idx;
ALTER TABLE invoices
  DROP CONSTRAINT invoices_void_check,
  DROP CONSTRAINT invoices_term_check,
  DROP CONSTRAINT invoices_issued_check,
  DROP CONSTRAINT invoices_note_length,
  DROP COLUMN void_reason,
  DROP COLUMN issued_by,
  DROP COLUMN issued_at,
  DROP COLUMN term_id,
  DROP COLUMN kind;
ALTER TABLE invoices ADD CONSTRAINT invoices_check CHECK (status = 'draft' OR number IS NOT NULL);
DROP TABLE fee_structure_instalments;
DROP TABLE fee_structure_lines;
DROP TABLE fee_structures;
DELETE FROM role_permissions WHERE permission_key = 'finance.payment.record';
DELETE FROM permissions WHERE key = 'finance.payment.record';
