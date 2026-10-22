-- Staff content manager (Stage 5). Each type has: Get (all editable fields
-- plus workflow state), List (summary), Insert, Update (editable fields),
-- SetStatus and Delete. Pending edits of published items live in
-- content_pending.

-- ---------- pending edits ----------

-- name: GetPending :one
SELECT document, updated_by, updated_at FROM content_pending WHERE entity_type = @entity_type AND entity_id = @entity_id;

-- name: UpsertPending :one
INSERT INTO content_pending (entity_type, entity_id, document, updated_by)
VALUES (@entity_type, @entity_id, @document, @updated_by)
ON CONFLICT (entity_type, entity_id) DO UPDATE SET document = EXCLUDED.document, updated_by = EXCLUDED.updated_by, updated_at = now()
RETURNING updated_at;

-- name: DeletePending :exec
DELETE FROM content_pending WHERE entity_type = @entity_type AND entity_id = @entity_id;

-- name: ListPendingIDs :many
SELECT entity_id FROM content_pending WHERE entity_type = @entity_type;

-- ---------- schools ----------

-- name: StaffGetSchool :one
SELECT id, slug, name, short_name, accent_key, summary, description, head_message, sort_order,
       status, published_at, is_sample, updated_at
FROM schools WHERE id = @id;

-- name: StaffListSchools :many
SELECT id, name AS label, status, is_sample, updated_at FROM schools ORDER BY sort_order, name;

-- name: StaffInsertSchool :one
INSERT INTO schools (slug, name, short_name, accent_key, summary, description, head_message, sort_order, created_by, updated_by)
VALUES (@slug, @name, @short_name, @accent_key, @summary, @description, @head_message, @sort_order, @actor, @actor)
RETURNING id;

-- name: StaffUpdateSchool :exec
UPDATE schools SET slug = @slug, name = @name, short_name = @short_name, accent_key = @accent_key, summary = @summary,
  description = @description, head_message = @head_message, sort_order = @sort_order, updated_by = @actor
WHERE id = @id;

-- name: StaffSetSchoolStatus :exec
UPDATE schools SET status = @status, published_at = @published_at, updated_by = @actor WHERE id = @id;

-- name: StaffDeleteSchool :exec
DELETE FROM schools WHERE id = @id;

-- ---------- programmes ----------

-- name: StaffGetProgramme :one
SELECT id, school_id, slug, title, level, mode, duration_months, duration_note, min_kcse_grade, summary, overview,
       what_you_learn, career_outcomes, assessment_model, seo_title, seo_description, sort_order,
       status, published_at, is_sample, updated_at
FROM programmes WHERE id = @id;

-- name: StaffListProgrammes :many
SELECT p.id, p.title AS label, p.status, p.is_sample, p.updated_at, s.short_name AS school_name
FROM programmes p JOIN schools s ON s.id = p.school_id
ORDER BY s.sort_order, p.sort_order, p.title;

-- name: StaffInsertProgramme :one
INSERT INTO programmes (school_id, slug, title, level, mode, duration_months, duration_note, min_kcse_grade, summary,
  overview, what_you_learn, career_outcomes, assessment_model, seo_title, seo_description, sort_order, created_by, updated_by)
VALUES (@school_id, @slug, @title, @level, @mode, @duration_months, @duration_note, @min_kcse_grade, @summary,
  @overview, @what_you_learn, @career_outcomes, @assessment_model, @seo_title, @seo_description, @sort_order, @actor, @actor)
RETURNING id;

-- name: StaffUpdateProgramme :exec
UPDATE programmes SET school_id = @school_id, slug = @slug, title = @title, level = @level, mode = @mode,
  duration_months = @duration_months, duration_note = @duration_note, min_kcse_grade = @min_kcse_grade,
  summary = @summary, overview = @overview, what_you_learn = @what_you_learn, career_outcomes = @career_outcomes,
  assessment_model = @assessment_model, seo_title = @seo_title, seo_description = @seo_description,
  sort_order = @sort_order, updated_by = @actor
WHERE id = @id;

-- name: StaffSetProgrammeStatus :exec
UPDATE programmes SET status = @status, published_at = @published_at, updated_by = @actor WHERE id = @id;

-- name: StaffDeleteProgramme :exec
DELETE FROM programmes WHERE id = @id;

-- name: ListProgrammeRequirements :many
SELECT kind, requirement FROM programme_requirements WHERE programme_id = @programme_id ORDER BY sort_order;

-- name: ListProgrammeIntakes :many
SELECT id, name, starts_on, applications_open_on, application_deadline, capacity, status
FROM programme_intakes WHERE programme_id = @programme_id ORDER BY starts_on;

-- name: InsertProgrammeIntake :one
INSERT INTO programme_intakes (programme_id, name, starts_on, applications_open_on, application_deadline, capacity, status)
VALUES (@programme_id, @name, @starts_on, @applications_open_on, @application_deadline, @capacity, @status)
RETURNING id;

-- name: UpdateProgrammeIntake :execrows
UPDATE programme_intakes SET name = @name, starts_on = @starts_on, applications_open_on = @applications_open_on,
  application_deadline = @application_deadline, capacity = @capacity, status = @status
WHERE id = @id AND programme_id = @programme_id;

-- name: DeleteProgrammeIntakesExcept :exec
-- Removes intakes not in the document. An intake that applications refer to
-- cannot be removed (foreign key), so the save fails instead of losing data.
DELETE FROM programme_intakes WHERE programme_id = @programme_id AND NOT (id = ANY(@keep_ids::uuid[]));

-- name: ListProgrammeFeeItems :many
SELECT label, category, amount_cents, frequency, notes FROM fee_items
WHERE programme_id = @programme_id AND intake_id IS NULL ORDER BY sort_order;

-- ---------- news ----------

-- name: StaffGetNewsPost :one
SELECT id, slug, title, summary, body, cover_file_id, cover_alt, seo_title, seo_description,
       status, published_at, is_sample, updated_at
FROM news_posts WHERE id = @id;

-- name: StaffListNewsPosts :many
SELECT id, title AS label, status, is_sample, updated_at FROM news_posts ORDER BY COALESCE(published_at, created_at) DESC;

-- name: StaffInsertNewsPost :one
INSERT INTO news_posts (slug, title, summary, body, cover_file_id, cover_alt, seo_title, seo_description, author_id)
VALUES (@slug, @title, @summary, @body, @cover_file_id, @cover_alt, @seo_title, @seo_description, @actor)
RETURNING id;

-- name: StaffUpdateNewsPost :exec
UPDATE news_posts SET slug = @slug, title = @title, summary = @summary, body = @body, cover_file_id = @cover_file_id,
  cover_alt = @cover_alt, seo_title = @seo_title, seo_description = @seo_description
WHERE id = @id;

-- name: StaffSetNewsPostStatus :exec
UPDATE news_posts SET status = @status, published_at = @published_at WHERE id = @id;

-- name: StaffDeleteNewsPost :exec
DELETE FROM news_posts WHERE id = @id;

-- ---------- events ----------

-- name: StaffGetEvent :one
SELECT id, slug, title, summary, body, location, starts_at, ends_at, status, published_at, is_sample, updated_at
FROM events WHERE id = @id;

-- name: StaffListEvents :many
SELECT id, title AS label, status, is_sample, updated_at FROM events ORDER BY starts_at DESC;

-- name: StaffInsertEvent :one
INSERT INTO events (slug, title, summary, body, location, starts_at, ends_at)
VALUES (@slug, @title, @summary, @body, @location, @starts_at, @ends_at)
RETURNING id;

-- name: StaffUpdateEvent :exec
UPDATE events SET slug = @slug, title = @title, summary = @summary, body = @body, location = @location,
  starts_at = @starts_at, ends_at = @ends_at
WHERE id = @id;

-- name: StaffSetEventStatus :exec
UPDATE events SET status = @status, published_at = @published_at WHERE id = @id;

-- name: StaffDeleteEvent :exec
DELETE FROM events WHERE id = @id;

-- ---------- FAQ categories (no workflow: structure, changed by publishers) ----------

-- name: StaffGetFaqCategory :one
SELECT id, slug, name, sort_order, is_sample FROM faq_categories WHERE id = @id;

-- name: StaffListFaqCategories :many
SELECT id, name AS label, is_sample FROM faq_categories ORDER BY sort_order, name;

-- name: StaffInsertFaqCategory :one
INSERT INTO faq_categories (slug, name, sort_order) VALUES (@slug, @name, @sort_order) RETURNING id;

-- name: StaffUpdateFaqCategory :exec
UPDATE faq_categories SET slug = @slug, name = @name, sort_order = @sort_order WHERE id = @id;

-- name: StaffDeleteFaqCategory :exec
DELETE FROM faq_categories WHERE id = @id;

-- ---------- FAQs ----------

-- name: StaffGetFaq :one
SELECT id, category_id, programme_id, question, answer, sort_order, status, published_at, is_sample, updated_at
FROM faqs WHERE id = @id;

-- name: StaffListFaqs :many
SELECT f.id, f.question AS label, f.status, f.is_sample, f.updated_at, c.name AS category_name
FROM faqs f JOIN faq_categories c ON c.id = f.category_id
ORDER BY c.sort_order, f.sort_order;

-- name: StaffInsertFaq :one
INSERT INTO faqs (category_id, programme_id, question, answer, sort_order)
VALUES (@category_id, @programme_id, @question, @answer, @sort_order)
RETURNING id;

-- name: StaffUpdateFaq :exec
UPDATE faqs SET category_id = @category_id, programme_id = @programme_id, question = @question, answer = @answer,
  sort_order = @sort_order
WHERE id = @id;

-- name: StaffSetFaqStatus :exec
UPDATE faqs SET status = @status, published_at = @published_at WHERE id = @id;

-- name: StaffDeleteFaq :exec
DELETE FROM faqs WHERE id = @id;

-- ---------- banners ----------

-- name: StaffGetBanner :one
SELECT id, message, link_label, link_href, starts_at, ends_at, status, is_sample, updated_at FROM banners WHERE id = @id;

-- name: StaffListBanners :many
SELECT id, message AS label, status, is_sample, updated_at FROM banners ORDER BY starts_at DESC;

-- name: StaffInsertBanner :one
INSERT INTO banners (message, link_label, link_href, starts_at, ends_at)
VALUES (@message, @link_label, @link_href, @starts_at, @ends_at)
RETURNING id;

-- name: StaffUpdateBanner :exec
UPDATE banners SET message = @message, link_label = @link_label, link_href = @link_href, starts_at = @starts_at,
  ends_at = @ends_at
WHERE id = @id;

-- name: StaffSetBannerStatus :exec
UPDATE banners SET status = @status WHERE id = @id;

-- name: StaffDeleteBanner :exec
DELETE FROM banners WHERE id = @id;

-- ---------- gallery ----------

-- name: StaffGetGalleryItem :one
SELECT id, file_id, caption, alt_text, consent_confirmed, consent_note, sort_order, status, is_sample, updated_at
FROM gallery_items WHERE id = @id;

-- name: StaffListGalleryItems :many
SELECT id, alt_text AS label, status, is_sample, updated_at FROM gallery_items ORDER BY sort_order, created_at;

-- name: StaffInsertGalleryItem :one
INSERT INTO gallery_items (file_id, caption, alt_text, consent_confirmed, consent_note, sort_order)
VALUES (@file_id, @caption, @alt_text, @consent_confirmed, @consent_note, @sort_order)
RETURNING id;

-- name: StaffUpdateGalleryItem :exec
UPDATE gallery_items SET file_id = @file_id, caption = @caption, alt_text = @alt_text,
  consent_confirmed = @consent_confirmed, consent_note = @consent_note, sort_order = @sort_order
WHERE id = @id;

-- name: StaffSetGalleryItemStatus :exec
UPDATE gallery_items SET status = @status WHERE id = @id;

-- name: StaffDeleteGalleryItem :exec
DELETE FROM gallery_items WHERE id = @id;

-- ---------- statistics ----------

-- name: StaffGetStatistic :one
SELECT id, label, value, source_note, verified_on, verified_by, sort_order, status, is_sample, updated_at
FROM statistics WHERE id = @id;

-- name: StaffListStatistics :many
SELECT id, label, status, is_sample, updated_at FROM statistics ORDER BY sort_order, label;

-- name: StaffInsertStatistic :one
INSERT INTO statistics (label, value, source_note, verified_on, verified_by, sort_order)
VALUES (@label, @value, @source_note, @verified_on, @verified_by, @sort_order)
RETURNING id;

-- name: StaffUpdateStatistic :exec
UPDATE statistics SET label = @label, value = @value, source_note = @source_note, verified_on = @verified_on,
  verified_by = @verified_by, sort_order = @sort_order
WHERE id = @id;

-- name: StaffSetStatisticStatus :exec
UPDATE statistics SET status = @status WHERE id = @id;

-- name: StaffDeleteStatistic :exec
DELETE FROM statistics WHERE id = @id;

-- ---------- accreditations ----------

-- name: StaffGetAccreditation :one
SELECT id, body, reference, description, evidence_note, verified_on, verified_by, sort_order, status, is_sample, updated_at
FROM accreditations WHERE id = @id;

-- name: StaffListAccreditations :many
SELECT id, body AS label, status, is_sample, updated_at FROM accreditations ORDER BY sort_order, body;

-- name: StaffInsertAccreditation :one
INSERT INTO accreditations (body, reference, description, evidence_note, verified_on, verified_by, sort_order)
VALUES (@body, @reference, @description, @evidence_note, @verified_on, @verified_by, @sort_order)
RETURNING id;

-- name: StaffUpdateAccreditation :exec
UPDATE accreditations SET body = @body, reference = @reference, description = @description,
  evidence_note = @evidence_note, verified_on = @verified_on, verified_by = @verified_by, sort_order = @sort_order
WHERE id = @id;

-- name: StaffSetAccreditationStatus :exec
UPDATE accreditations SET status = @status WHERE id = @id;

-- name: StaffDeleteAccreditation :exec
DELETE FROM accreditations WHERE id = @id;

-- ---------- site settings ----------

-- name: StaffListSiteSettings :many
SELECT key, value, description, updated_at FROM site_settings ORDER BY key;

-- name: StaffUpdateSiteSetting :execrows
UPDATE site_settings SET value = @value, updated_by = @actor WHERE key = @key;

-- ---------- files ----------

-- name: InsertImageFile :one
INSERT INTO files (bucket, object_key, original_name, content_type, size_bytes, sha256, uploaded_by_user_id, width, height, image_widths)
VALUES (@bucket, @object_key, @original_name, @content_type, @size_bytes, @sha256, @uploaded_by_user_id, @width, @height, @image_widths)
RETURNING id, created_at;

-- name: GetFile :one
SELECT id, bucket, object_key, original_name, content_type, size_bytes, width, height, image_widths, created_at
FROM files WHERE id = @id AND deleted_at IS NULL;

-- ---------- pages ----------

-- name: StaffGetPage :one
SELECT id, slug, title, summary, body, seo_title, seo_description, status, published_at, is_sample, updated_at
FROM pages WHERE id = @id;

-- name: StaffListPages :many
SELECT id, title AS label, slug, status, is_sample, updated_at FROM pages ORDER BY title;

-- name: StaffInsertPage :one
INSERT INTO pages (slug, title, summary, body, seo_title, seo_description, created_by, updated_by)
VALUES (@slug, @title, @summary, @body, @seo_title, @seo_description, @actor, @actor)
RETURNING id;

-- name: StaffUpdatePage :exec
UPDATE pages SET slug = @slug, title = @title, summary = @summary, body = @body, seo_title = @seo_title,
  seo_description = @seo_description, updated_by = @actor
WHERE id = @id;

-- name: StaffSetPageStatus :exec
UPDATE pages SET status = @status, published_at = @published_at, updated_by = @actor WHERE id = @id;

-- name: StaffDeletePage :exec
DELETE FROM pages WHERE id = @id;
