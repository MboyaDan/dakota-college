package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Summary counts what one run wrote.
type Summary struct {
	Schools, Programmes, Intakes, FeeItems, FAQs, Settings, Pages, News, Events, Users int
}

// Run writes the sample data in one transaction. It is idempotent: running it
// again updates the same rows instead of adding duplicates. Call Guard first.
func Run(ctx context.Context, pool *pgxpool.Pool, password string) (Summary, error) {
	var sum Summary
	if len(password) < auth.MinPasswordLength {
		return sum, fmt.Errorf("SEED_PASSWORD must be at least %d characters", auth.MinPasswordLength)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return sum, err
	}

	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		now := time.Now()

		for i, s := range schoolSeeds {
			schoolID, err := q.UpsertSchool(ctx, dbq.UpsertSchoolParams{
				Slug: s.slug, Name: s.name, ShortName: s.shortName, AccentKey: s.accent,
				Summary: s.summary, SortOrder: int32(i), Status: "published", PublishedAt: &now, IsSample: true,
			})
			if err != nil {
				return fmt.Errorf("school %s: %w", s.slug, err)
			}
			sum.Schools++

			for j, p := range s.programmes {
				status, publishedAt := "draft", (*time.Time)(nil)
				if p.published {
					status, publishedAt = "published", &now
				}
				var minGrade *string
				if p.minGrade != "" {
					minGrade = &p.minGrade
				}
				months := p.months
				programmeID, err := q.UpsertProgramme(ctx, dbq.UpsertProgrammeParams{
					SchoolID: schoolID, Slug: p.slug, Title: p.title, Level: p.level, Mode: "full_time",
					DurationMonths: &months, MinKcseGrade: minGrade, Summary: p.summary,
					Overview:       "(Sample) Overview text written by the college.",
					WhatYouLearn:   "(Sample) What students learn.",
					CareerOutcomes: "(Sample) Possible careers.",
					SortOrder:      int32(j), Status: status, PublishedAt: publishedAt, IsSample: true,
				})
				if err != nil {
					return fmt.Errorf("programme %s: %w", p.slug, err)
				}
				sum.Programmes++

				if err := seedProgrammeDetails(ctx, q, programmeID, &sum); err != nil {
					return fmt.Errorf("programme %s details: %w", p.slug, err)
				}
			}
		}

		if err := q.DeleteSampleFaqs(ctx); err != nil {
			return err
		}
		for i, c := range faqSeeds {
			catID, err := q.UpsertFaqCategory(ctx, dbq.UpsertFaqCategoryParams{Slug: c.categorySlug, Name: c.categoryName, SortOrder: int32(i), IsSample: true})
			if err != nil {
				return fmt.Errorf("faq category %s: %w", c.categorySlug, err)
			}
			for j, qa := range c.items {
				if err := q.InsertFaq(ctx, dbq.InsertFaqParams{
					CategoryID: catID, Question: qa[0], Answer: qa[1], SortOrder: int32(j),
					Status: "published", PublishedAt: &now, IsSample: true,
				}); err != nil {
					return err
				}
				sum.FAQs++
			}
		}

		for key, value := range settingSeeds {
			n, err := q.SetSiteSetting(ctx, dbq.SetSiteSettingParams{Key: key, Value: value})
			if err != nil {
				return err
			}
			if n != 1 {
				return fmt.Errorf("site setting %q does not exist; add it in a migration first", key)
			}
			sum.Settings++
		}

		// Stays a draft: unverified, so it can never be published (NN-01).
		if err := q.DeleteSampleAccreditations(ctx); err != nil {
			return err
		}
		if err := q.InsertAccreditation(ctx, dbq.InsertAccreditationParams{
			Body: "(Sample) Registration body", Reference: "SAMPLE-0000, not a real registration number",
			Description: "(Sample) Placeholder until the college supplies verified details.", Status: "draft", IsSample: true,
		}); err != nil {
			return err
		}

		pages := append([]pageSeed(nil), pageSeeds...)
		for _, l := range legalPages {
			body, err := legalDrafts.ReadFile("legal/" + l.file)
			if err != nil {
				return err
			}
			pages = append(pages, pageSeed{slug: l.slug, title: "(Sample) Draft: " + l.title, body: draftNote + string(body)})
		}
		for _, pg := range pages {
			if err := q.UpsertPage(ctx, dbq.UpsertPageParams{Slug: pg.slug, Title: pg.title, Summary: pg.summary, Body: pg.body,
				Status: "published", PublishedAt: &now, IsSample: true}); err != nil {
				return fmt.Errorf("page %s: %w", pg.slug, err)
			}
			sum.Pages++
		}

		// News and events dated relative to now, so there is always one
		// upcoming and one past event to look at.
		for i, n := range newsSeeds {
			at := now.Add(-time.Duration(i+1) * 24 * time.Hour)
			if err := q.UpsertNewsPost(ctx, dbq.UpsertNewsPostParams{Slug: n.slug, Title: n.title, Summary: n.summary, Body: n.body,
				PublishedAt: &at, IsSample: true}); err != nil {
				return fmt.Errorf("news %s: %w", n.slug, err)
			}
			sum.News++
		}
		for _, e := range eventSeeds {
			starts := now.Add(e.startsIn).Truncate(time.Hour)
			ends := starts.Add(3 * time.Hour)
			if err := q.UpsertEvent(ctx, dbq.UpsertEventParams{Slug: e.slug, Title: e.title, Summary: e.summary, Body: e.body,
				Location: e.location, StartsAt: starts, EndsAt: &ends, PublishedAt: &now, IsSample: true}); err != nil {
				return fmt.Errorf("event %s: %w", e.slug, err)
			}
			sum.Events++
		}

		for _, u := range userSeeds {
			userID, err := q.UpsertUserByEmail(ctx, dbq.UpsertUserByEmailParams{Email: u.email, FullName: u.name, Kind: u.kind, PasswordHash: &hash})
			if err != nil {
				return fmt.Errorf("user %s: %w", u.email, err)
			}
			role, err := q.GetRoleByKey(ctx, u.role)
			if err != nil {
				return fmt.Errorf("role %s: %w", u.role, err)
			}
			if err := q.AssignRole(ctx, dbq.AssignRoleParams{UserID: userID, RoleID: role.ID}); err != nil {
				return err
			}
			sum.Users++
		}
		return nil
	})
	if err != nil {
		return Summary{}, err
	}
	return sum, nil
}

func seedProgrammeDetails(ctx context.Context, q *dbq.Queries, programmeID uuid.UUID, sum *Summary) error {
	if err := q.DeleteProgrammeRequirements(ctx, programmeID); err != nil {
		return err
	}
	for i, r := range [][2]string{
		{"academic", "(Sample) KCSE mean grade or equivalent, as set by the college."},
		{"document", "(Sample) Copy of national ID or birth certificate."},
		{"document", "(Sample) Copy of KCSE certificate or result slip."},
	} {
		if err := q.InsertProgrammeRequirement(ctx, dbq.InsertProgrammeRequirementParams{
			ProgrammeID: programmeID, Kind: r[0], Requirement: r[1], SortOrder: int32(i),
		}); err != nil {
			return err
		}
	}

	type intake struct {
		name             string
		starts, deadline time.Time
		status           string
	}
	intakes := []intake{
		{"January 2027 (Sample)", date(2027, 1, 11), date(2026, 12, 15), "open"},
		{"May 2027 (Sample)", date(2027, 5, 10), date(2027, 4, 15), "planned"},
	}
	for _, in := range intakes {
		deadline := in.deadline
		_, err := q.UpsertIntake(ctx, dbq.UpsertIntakeParams{
			ProgrammeID: programmeID, Name: in.name, StartsOn: in.starts, ApplicationDeadline: &deadline,
			Status: in.status, IsSample: true,
		})
		if err != nil {
			return err
		}
		sum.Intakes++
	}

	if err := q.DeleteFeeItems(ctx, programmeID); err != nil {
		return err
	}
	for i, f := range []struct {
		label, category, frequency string
		cents                      int64
	}{
		{"(Sample) Tuition", "tuition", "per_term", 1_000_000},
		{"(Sample) Registration", "registration", "once", 100_000},
		{"(Sample) Assessment", "assessment", "per_year", 250_000},
	} {
		if err := q.InsertFeeItem(ctx, dbq.InsertFeeItemParams{
			ProgrammeID: programmeID, Label: f.label, Category: f.category, AmountCents: f.cents,
			Frequency: f.frequency, Notes: "(Sample) Not a real fee.", SortOrder: int32(i), IsSample: true,
		}); err != nil {
			return err
		}
		sum.FeeItems++
	}
	return nil
}

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
