-- ---------- dashboards and count exports (Stage 16b) ----------
-- Dates are Nairobi dates; periods are inclusive.

-- name: ProgrammeSchools :many
SELECT p.id AS programme_id, s.id AS school_id, s.name AS school_name
FROM programmes p JOIN schools s ON s.id = p.school_id;

-- name: StudentCounts :many
-- Students by school, programme and status, with how many joined in the period.
SELECT s.id AS school_id, s.name AS school_name, p.id AS programme_id, p.title AS programme_title, st.status,
       count(*)::bigint AS students,
       count(*) FILTER (WHERE (st.created_at AT TIME ZONE 'Africa/Nairobi')::date BETWEEN @from_date::date AND @to_date::date)::bigint AS joined
FROM students st
JOIN programmes p ON p.id = st.programme_id
JOIN schools s ON s.id = p.school_id
GROUP BY s.id, s.name, p.id, p.title, st.status
ORDER BY s.name, p.title, st.status;

-- name: ResultSheetCounts :one
SELECT count(*) FILTER (WHERE status = 'draft')::bigint AS draft,
       count(*) FILTER (WHERE status = 'submitted')::bigint AS submitted,
       count(*) FILTER (WHERE status = 'approved')::bigint AS approved,
       count(*) FILTER (WHERE status = 'released')::bigint AS released
FROM result_sheets;

-- name: ReleasedResults :many
-- The newest release of each class whose newest release falls in the period.
SELECT t.name AS term_name, t.starts_on AS term_starts_on, u.code AS unit_code, u.title AS unit_title,
       sc.name AS school_name, r.version, r.released_at,
       jsonb_array_length(r.results)::bigint AS results,
       (SELECT count(*) FROM jsonb_array_elements(r.results) e
        WHERE (e->>'passed')::boolean IS TRUE OR e->>'outcome' = 'competent')::bigint AS passed
FROM (
  SELECT DISTINCT ON (sheet_id) * FROM result_releases ORDER BY sheet_id, version DESC
) r
JOIN result_sheets rs ON rs.id = r.sheet_id
JOIN unit_offerings o ON o.id = rs.offering_id
JOIN academic_terms t ON t.id = o.term_id
JOIN units u ON u.id = o.unit_id
JOIN schools sc ON sc.id = u.school_id
WHERE (r.released_at AT TIME ZONE 'Africa/Nairobi')::date BETWEEN @from_date::date AND @to_date::date
ORDER BY t.starts_on, u.code;

-- name: FinanceByMonth :many
-- Billed (issued invoices), collected (confirmed payments by the day paid),
-- charges and credits, by month of the period.
WITH m AS (
  SELECT to_char((i.issued_at AT TIME ZONE 'Africa/Nairobi')::date, 'YYYY-MM') AS month, sum(l.amount_cents) AS billed, 0::numeric AS collected, 0::numeric AS charges, 0::numeric AS credits
  FROM invoices i JOIN invoice_lines l ON l.invoice_id = i.id
  WHERE i.status = 'issued' AND (i.issued_at AT TIME ZONE 'Africa/Nairobi')::date BETWEEN @from_date::date AND @to_date::date
  GROUP BY 1
  UNION ALL
  SELECT to_char(paid_on, 'YYYY-MM'), 0, sum(amount_cents), 0, 0
  FROM payments WHERE status = 'confirmed' AND paid_on BETWEEN @from_date::date AND @to_date::date
  GROUP BY 1
  UNION ALL
  SELECT to_char((created_at AT TIME ZONE 'Africa/Nairobi')::date, 'YYYY-MM'), 0, 0,
         sum(amount_cents) FILTER (WHERE kind = 'charge'), sum(amount_cents) FILTER (WHERE kind = 'credit')
  FROM adjustments WHERE (created_at AT TIME ZONE 'Africa/Nairobi')::date BETWEEN @from_date::date AND @to_date::date
  GROUP BY 1
)
SELECT month::text AS month,
       coalesce(sum(billed), 0)::bigint AS billed_cents,
       coalesce(sum(collected), 0)::bigint AS collected_cents,
       coalesce(sum(charges), 0)::bigint AS charges_cents,
       coalesce(sum(credits), 0)::bigint AS credits_cents
FROM m GROUP BY month ORDER BY month;

-- name: EnquiryCounts :many
-- Enquiries by topic: received and closed in the period, and still open now.
SELECT query_type,
       count(*) FILTER (WHERE (created_at AT TIME ZONE 'Africa/Nairobi')::date BETWEEN @from_date::date AND @to_date::date)::bigint AS received,
       count(*) FILTER (WHERE (closed_at AT TIME ZONE 'Africa/Nairobi')::date BETWEEN @from_date::date AND @to_date::date)::bigint AS closed,
       count(*) FILTER (WHERE status = 'new')::bigint AS new_now,
       count(*) FILTER (WHERE status IN ('open', 'waiting'))::bigint AS open_now
FROM enquiries GROUP BY query_type ORDER BY query_type;

-- name: StudentServiceCounts :one
SELECT (SELECT count(*) FROM service_requests WHERE status IN ('open', 'in_progress'))::bigint AS open_requests,
       (SELECT count(*) FROM profile_change_requests WHERE status = 'pending')::bigint AS pending_changes;
