-- name: UpsertSchool :one
INSERT INTO schools (slug, name, short_name, accent_key, summary, description, sort_order, status, published_at, is_sample)
VALUES (@slug, @name, @short_name, @accent_key, @summary, @description, @sort_order, @status, @published_at, @is_sample)
ON CONFLICT (slug) DO UPDATE
  SET name = EXCLUDED.name, short_name = EXCLUDED.short_name, accent_key = EXCLUDED.accent_key,
      summary = EXCLUDED.summary, description = EXCLUDED.description, sort_order = EXCLUDED.sort_order,
      status = EXCLUDED.status, published_at = EXCLUDED.published_at, is_sample = EXCLUDED.is_sample
RETURNING id;

-- name: UpsertProgramme :one
INSERT INTO programmes (
  school_id, slug, title, level, mode, duration_months, duration_note, min_kcse_grade,
  summary, overview, what_you_learn, career_outcomes, sort_order, status, published_at, is_sample)
VALUES (
  @school_id, @slug, @title, @level, @mode, @duration_months, @duration_note, @min_kcse_grade,
  @summary, @overview, @what_you_learn, @career_outcomes, @sort_order, @status, @published_at, @is_sample)
ON CONFLICT (slug) DO UPDATE
  SET school_id = EXCLUDED.school_id, title = EXCLUDED.title, level = EXCLUDED.level, mode = EXCLUDED.mode,
      duration_months = EXCLUDED.duration_months, duration_note = EXCLUDED.duration_note,
      min_kcse_grade = EXCLUDED.min_kcse_grade, summary = EXCLUDED.summary, overview = EXCLUDED.overview,
      what_you_learn = EXCLUDED.what_you_learn, career_outcomes = EXCLUDED.career_outcomes,
      sort_order = EXCLUDED.sort_order, status = EXCLUDED.status, published_at = EXCLUDED.published_at,
      is_sample = EXCLUDED.is_sample
RETURNING id;

-- name: DeleteProgrammeRequirements :exec
DELETE FROM programme_requirements WHERE programme_id = @programme_id;

-- name: InsertProgrammeRequirement :exec
INSERT INTO programme_requirements (programme_id, kind, requirement, sort_order)
VALUES (@programme_id, @kind, @requirement, @sort_order);

-- name: UpsertIntake :one
INSERT INTO programme_intakes (programme_id, name, starts_on, applications_open_on, application_deadline, status, is_sample)
VALUES (@programme_id, @name, @starts_on, @applications_open_on, @application_deadline, @status, @is_sample)
ON CONFLICT (programme_id, starts_on) DO UPDATE
  SET name = EXCLUDED.name, applications_open_on = EXCLUDED.applications_open_on,
      application_deadline = EXCLUDED.application_deadline, status = EXCLUDED.status, is_sample = EXCLUDED.is_sample
RETURNING id;

-- name: DeleteFeeItems :exec
DELETE FROM fee_items WHERE programme_id = @programme_id;

-- name: InsertFeeItem :exec
INSERT INTO fee_items (programme_id, intake_id, label, category, amount_cents, frequency, notes, sort_order, is_sample)
VALUES (@programme_id, @intake_id, @label, @category, @amount_cents, @frequency, @notes, @sort_order, @is_sample);

-- name: UpsertFaqCategory :one
INSERT INTO faq_categories (slug, name, sort_order, is_sample)
VALUES (@slug, @name, @sort_order, @is_sample)
ON CONFLICT (slug) DO UPDATE SET name = EXCLUDED.name, sort_order = EXCLUDED.sort_order, is_sample = EXCLUDED.is_sample
RETURNING id;

-- name: DeleteSampleFaqs :exec
DELETE FROM faqs WHERE is_sample;

-- name: InsertFaq :exec
INSERT INTO faqs (category_id, question, answer, sort_order, status, published_at, is_sample)
VALUES (@category_id, @question, @answer, @sort_order, @status, @published_at, @is_sample);

-- name: DeleteSampleAccreditations :exec
DELETE FROM accreditations WHERE is_sample;

-- name: InsertAccreditation :exec
INSERT INTO accreditations (body, reference, description, status, is_sample)
VALUES (@body, @reference, @description, @status, @is_sample);

-- name: SetSiteSetting :execrows
UPDATE site_settings SET value = @value WHERE key = @key;

-- name: ListSiteSettings :many
SELECT key, value FROM site_settings ORDER BY key;

-- name: ListPublishedProgrammes :many
SELECT p.id, p.slug, p.title, p.level, p.mode, p.duration_months, p.min_kcse_grade, p.summary,
       s.slug AS school_slug, s.short_name AS school_name, s.accent_key
FROM programmes p
JOIN schools s ON s.id = p.school_id
WHERE p.status = 'published' AND s.status = 'published'
ORDER BY s.sort_order, p.sort_order, p.title;

-- name: CountSampleRows :one
-- Launch check (AC-17): every count must be zero in production.
SELECT
  (SELECT count(*) FROM schools WHERE is_sample)
  + (SELECT count(*) FROM programmes WHERE is_sample)
  + (SELECT count(*) FROM programme_intakes WHERE is_sample)
  + (SELECT count(*) FROM fee_items WHERE is_sample)
  + (SELECT count(*) FROM faq_categories WHERE is_sample)
  + (SELECT count(*) FROM faqs WHERE is_sample)
  + (SELECT count(*) FROM news_posts WHERE is_sample)
  + (SELECT count(*) FROM events WHERE is_sample)
  + (SELECT count(*) FROM banners WHERE is_sample)
  + (SELECT count(*) FROM gallery_items WHERE is_sample)
  + (SELECT count(*) FROM statistics WHERE is_sample)
  + (SELECT count(*) FROM accreditations WHERE is_sample)
  + (SELECT count(*) FROM academic_terms WHERE is_sample)
  + (SELECT count(*) FROM units WHERE is_sample)
  + (SELECT count(*) FROM pages WHERE is_sample)
  AS total;

-- name: UpsertPage :exec
INSERT INTO pages (slug, title, summary, body, status, published_at, is_sample)
VALUES (@slug, @title, @summary, @body, @status, @published_at, @is_sample)
ON CONFLICT (slug) DO UPDATE
  SET title = EXCLUDED.title, summary = EXCLUDED.summary, body = EXCLUDED.body, status = EXCLUDED.status,
      published_at = EXCLUDED.published_at, is_sample = EXCLUDED.is_sample;

-- name: UpsertNewsPost :exec
INSERT INTO news_posts (slug, title, summary, body, status, published_at, is_sample)
VALUES (@slug, @title, @summary, @body, 'published', @published_at, @is_sample)
ON CONFLICT (slug) DO UPDATE
  SET title = EXCLUDED.title, summary = EXCLUDED.summary, body = EXCLUDED.body, status = EXCLUDED.status,
      published_at = EXCLUDED.published_at, is_sample = EXCLUDED.is_sample;

-- name: UpsertEvent :exec
INSERT INTO events (slug, title, summary, body, location, starts_at, ends_at, status, published_at, is_sample)
VALUES (@slug, @title, @summary, @body, @location, @starts_at, @ends_at, 'published', @published_at, @is_sample)
ON CONFLICT (slug) DO UPDATE
  SET title = EXCLUDED.title, summary = EXCLUDED.summary, body = EXCLUDED.body, location = EXCLUDED.location,
      starts_at = EXCLUDED.starts_at, ends_at = EXCLUDED.ends_at, status = EXCLUDED.status,
      published_at = EXCLUDED.published_at, is_sample = EXCLUDED.is_sample;

-- name: DeleteStaleSampleIntakes :exec
-- Dev seed only: sample intakes from earlier runs that no application uses.
DELETE FROM programme_intakes i
WHERE i.programme_id = @programme_id AND i.is_sample AND NOT (i.starts_on = ANY(@keep::date[]))
  AND NOT EXISTS (SELECT 1 FROM applications a WHERE a.intake_id = i.id)
  AND NOT EXISTS (SELECT 1 FROM fee_items f WHERE f.intake_id = i.id);
