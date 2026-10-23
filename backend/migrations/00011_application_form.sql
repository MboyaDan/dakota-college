-- Online application form, part 1 (Stage 9a): the fields the wizard saves as
-- the applicant goes. Documents, declaration and submission are Stage 9b.

-- +goose Up
ALTER TABLE applicants
  ADD COLUMN identity_document_type text
    CHECK (identity_document_type IN ('national_id', 'passport', 'birth_certificate'));

-- KCSE (or equivalent) subject grades, as entered: [{"subject": "...", "grade": "..."}].
ALTER TABLE application_education
  ADD COLUMN subject_grades jsonb NOT NULL DEFAULT '[]'::jsonb
    CHECK (jsonb_typeof(subject_grades) = 'array' AND jsonb_array_length(subject_grades) <= 20);

-- One guardian per application.
CREATE UNIQUE INDEX application_guardians_one_idx ON application_guardians (application_id);

-- Where to go after an applicant signs in with a link (an /apply/... path).
ALTER TABLE applicant_sign_in_links
  ADD COLUMN next_path text NOT NULL DEFAULT ''
    CHECK (next_path = '' OR (next_path LIKE '/apply/%' AND length(next_path) <= 300));

-- +goose Down
ALTER TABLE applicant_sign_in_links DROP COLUMN next_path;
DROP INDEX application_guardians_one_idx;
ALTER TABLE application_education DROP COLUMN subject_grades;
ALTER TABLE applicants DROP COLUMN identity_document_type;
