-- Public website reads (Stage 6). Only published content, and only content
-- whose parent is published too (a programme of an unpublished school is
-- hidden). Child rows are read for all programmes at once, not per programme.

-- name: PublicListSettings :many
-- Only keys meant for visitors; internal settings (number formats, expiry
-- days) stay private.
SELECT key, value FROM site_settings
WHERE key LIKE 'contact.%' OR key LIKE 'campus.%' OR key LIKE 'about.%' OR key LIKE 'social.%'
ORDER BY key;

-- name: PublicListActiveBanners :many
SELECT message, link_label, link_href, ends_at, is_sample FROM banners
WHERE status = 'published' AND starts_at <= now() AND ends_at > now()
ORDER BY starts_at DESC;

-- name: PublicListSchools :many
-- A photo is published only with consent (table check, NN-08).
SELECT slug, name, short_name, accent_key, summary, description, head_message, photo_file_id, photo_alt, is_sample
FROM schools
WHERE status = 'published'
ORDER BY sort_order, name;

-- name: PublicListProgrammes :many
SELECT p.id, p.slug, p.title, s.slug AS school_slug, p.level, p.mode, p.duration_months, p.duration_note,
       p.min_kcse_grade, p.summary, p.overview, p.what_you_learn, p.career_outcomes, p.seo_title, p.seo_description,
       p.published_at, p.updated_at, p.is_sample
FROM programmes p
JOIN schools s ON s.id = p.school_id
WHERE p.status = 'published' AND s.status = 'published'
ORDER BY s.sort_order, p.sort_order, p.title;

-- name: PublicListRequirements :many
SELECT r.programme_id, r.kind, r.requirement
FROM programme_requirements r
JOIN programmes p ON p.id = r.programme_id
WHERE p.status = 'published'
ORDER BY r.programme_id, r.sort_order;

-- name: PublicListUpcomingIntakes :many
-- Intakes that have not started yet (today is the Nairobi date) and are not
-- closed.
SELECT i.id, i.programme_id, i.name, i.starts_on, i.applications_open_on, i.application_deadline, i.status
FROM programme_intakes i
JOIN programmes p ON p.id = i.programme_id
WHERE p.status = 'published' AND i.status <> 'closed' AND i.starts_on >= @today::date
ORDER BY i.programme_id, i.starts_on;

-- name: PublicListFees :many
SELECT f.programme_id, f.intake_id, f.label, f.category, f.amount_cents, f.frequency, f.notes
FROM fee_items f
JOIN programmes p ON p.id = f.programme_id
WHERE p.status = 'published'
ORDER BY f.programme_id, f.sort_order;

-- name: PublicListFaqs :many
SELECT c.slug AS category_slug, c.name AS category_name, f.programme_id, f.question, f.answer, f.is_sample
FROM faqs f
JOIN faq_categories c ON c.id = f.category_id
LEFT JOIN programmes p ON p.id = f.programme_id
WHERE f.status = 'published' AND (f.programme_id IS NULL OR p.status = 'published')
ORDER BY c.sort_order, c.name, f.sort_order;

-- name: PublicListStatistics :many
-- The table refuses to publish a figure without a verification date and
-- source (AC-16), so every row here is verified.
SELECT label, value, verified_on, is_sample FROM statistics
WHERE status = 'published'
ORDER BY sort_order;

-- name: PublicListAccreditations :many
SELECT body, reference, description, verified_on, is_sample FROM accreditations
WHERE status = 'published'
ORDER BY sort_order;

-- name: PublicListUpcomingEvents :many
SELECT slug, title, summary, location, starts_at, ends_at, is_sample FROM events
WHERE status = 'published' AND coalesce(ends_at, starts_at) >= now()
ORDER BY starts_at
LIMIT @max_items;

-- name: PublicListLatestNews :many
SELECT slug, title, summary, published_at, cover_file_id, cover_alt, is_sample FROM news_posts
WHERE status = 'published'
ORDER BY published_at DESC
LIMIT @max_items;

-- name: PublicListPages :many
SELECT slug, title, summary, body, seo_title, seo_description, published_at, updated_at, is_sample FROM pages
WHERE status = 'published'
ORDER BY slug;

-- name: PublicListGallery :many
-- Every published photo has written consent (table check, NN-08).
SELECT g.file_id, g.caption, g.alt_text, g.is_sample FROM gallery_items g
WHERE g.status = 'published'
ORDER BY g.sort_order, g.created_at;

-- name: PublicGetHomeHero :one
-- The home page photo: a published gallery photo (so with consent) chosen
-- for the hero. If several are chosen, the most recently changed one wins.
SELECT file_id, caption, alt_text, is_sample FROM gallery_items
WHERE status = 'published' AND home_hero
ORDER BY updated_at DESC
LIMIT 1;

-- name: PublicListFounders :many
-- Every published profile has the person's consent (table check).
SELECT name, role_title, intro, story, photo_file_id, photo_alt, is_sample FROM founders
WHERE status = 'published'
ORDER BY sort_order, name;

-- name: PublicCountNews :one
SELECT count(*) FROM news_posts WHERE status = 'published';

-- name: PublicListNewsPage :many
SELECT slug, title, summary, published_at, cover_file_id, cover_alt, is_sample FROM news_posts
WHERE status = 'published'
ORDER BY published_at DESC
LIMIT @max_items OFFSET @skip;

-- name: PublicGetNewsPost :one
SELECT slug, title, summary, body, published_at, updated_at, cover_file_id, cover_alt, seo_title, seo_description, is_sample
FROM news_posts
WHERE slug = @slug AND status = 'published';

-- name: PublicListPastEvents :many
SELECT slug, title, summary, location, starts_at, ends_at, is_sample FROM events
WHERE status = 'published' AND coalesce(ends_at, starts_at) < now()
ORDER BY starts_at DESC
LIMIT @max_items;

-- name: PublicListAllUpcomingEvents :many
SELECT slug, title, summary, location, starts_at, ends_at, is_sample FROM events
WHERE status = 'published' AND coalesce(ends_at, starts_at) >= now()
ORDER BY starts_at;

-- name: PublicGetEvent :one
SELECT slug, title, summary, body, location, starts_at, ends_at, published_at, updated_at, is_sample FROM events
WHERE slug = @slug AND status = 'published';
