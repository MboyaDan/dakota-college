package content

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
)

// Public site snapshot (Stage 6). One document holds everything published
// that the public pages need, so the site caches one response and bundles one
// fallback copy for when the API is asleep (ADR-03, AC-15). News and event
// bodies are not included; their pages read them separately (Stage 7).

// nairobi is Kenya time. Kenya has no daylight saving, and a fixed zone does
// not depend on tzdata in the container image (D-64).
var nairobi = time.FixedZone("EAT", 3*60*60)

// homeListLimit caps the news and events in the snapshot.
const homeListLimit = 6

type Site struct {
	GeneratedAt    time.Time             `json:"generated_at"`
	Settings       map[string]string     `json:"settings"`
	Banners        []PublicBanner        `json:"banners"`
	Schools        []PublicSchool        `json:"schools"`
	Programmes     []PublicProgramme     `json:"programmes"`
	FAQs           []PublicFAQ           `json:"faqs"`
	Statistics     []PublicStatistic     `json:"statistics"`
	Accreditations []PublicAccreditation `json:"accreditations"`
	Events         []PublicEvent         `json:"events"`
	News           []PublicNews          `json:"news"`
	Pages          []PublicPage          `json:"pages"`
	Gallery        []PublicPhoto         `json:"gallery"`
}

type PublicPage struct {
	Slug           string     `json:"slug"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	Body           string     `json:"body"`
	SEOTitle       string     `json:"seo_title"`
	SEODescription string     `json:"seo_description"`
	PublishedAt    *time.Time `json:"published_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	IsSample       bool       `json:"is_sample"`
}

// PublicPhoto is a gallery photo. Only photos with recorded consent are
// published (NN-08).
type PublicPhoto struct {
	Image    *media.ImageFile `json:"image"`
	Caption  string           `json:"caption"`
	AltText  string           `json:"alt_text"`
	IsSample bool             `json:"is_sample"`
}

type PublicBanner struct {
	Message   string    `json:"message"`
	LinkLabel *string   `json:"link_label"`
	LinkHref  *string   `json:"link_href"`
	EndsAt    time.Time `json:"ends_at"`
	IsSample  bool      `json:"is_sample"`
}

type PublicSchool struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	ShortName   string `json:"short_name"`
	AccentKey   string `json:"accent_key"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	HeadMessage string `json:"head_message"`
	IsSample    bool   `json:"is_sample"`
}

type PublicProgramme struct {
	Slug           string         `json:"slug"`
	Title          string         `json:"title"`
	SchoolSlug     string         `json:"school_slug"`
	Level          string         `json:"level"`
	Mode           string         `json:"mode"`
	DurationMonths *int32         `json:"duration_months"`
	DurationNote   string         `json:"duration_note"`
	MinKCSEGrade   *string        `json:"min_kcse_grade"`
	Summary        string         `json:"summary"`
	Overview       string         `json:"overview"`
	WhatYouLearn   string         `json:"what_you_learn"`
	CareerOutcomes string         `json:"career_outcomes"`
	SEOTitle       string         `json:"seo_title"`
	SEODescription string         `json:"seo_description"`
	Requirements   []Requirement  `json:"requirements"`
	Intakes        []PublicIntake `json:"intakes"`
	Fees           []PublicFee    `json:"fees"`
	PublishedAt    *time.Time     `json:"published_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	IsSample       bool           `json:"is_sample"`
	id             uuid.UUID
}

type PublicIntake struct {
	ID                  uuid.UUID `json:"id"`
	Name                string    `json:"name"`
	StartsOn            Date      `json:"starts_on"`
	ApplicationsOpenOn  *Date     `json:"applications_open_on"`
	ApplicationDeadline *Date     `json:"application_deadline"`
	Status              string    `json:"status"`
}

type PublicFee struct {
	// IntakeID is set when the line applies to one intake only.
	IntakeID *uuid.UUID `json:"intake_id"`
	Label    string     `json:"label"`
	Category string     `json:"category"`
	// AmountCents is KES cents (ADR-05).
	AmountCents int64  `json:"amount_cents"`
	Frequency   string `json:"frequency"`
	Notes       string `json:"notes"`
}

type PublicFAQ struct {
	CategorySlug string `json:"category_slug"`
	CategoryName string `json:"category_name"`
	// ProgrammeSlug is set for questions about one programme.
	ProgrammeSlug *string `json:"programme_slug"`
	Question      string  `json:"question"`
	Answer        string  `json:"answer"`
	IsSample      bool    `json:"is_sample"`
}

type PublicStatistic struct {
	Label      string `json:"label"`
	Value      string `json:"value"`
	VerifiedOn Date   `json:"verified_on"`
	IsSample   bool   `json:"is_sample"`
}

type PublicAccreditation struct {
	Body        string `json:"body"`
	Reference   string `json:"reference"`
	Description string `json:"description"`
	VerifiedOn  Date   `json:"verified_on"`
	IsSample    bool   `json:"is_sample"`
}

type PublicEvent struct {
	Slug     string     `json:"slug"`
	Title    string     `json:"title"`
	Summary  string     `json:"summary"`
	Location string     `json:"location"`
	StartsAt time.Time  `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at"`
	IsSample bool       `json:"is_sample"`
}

type PublicNews struct {
	Slug        string           `json:"slug"`
	Title       string           `json:"title"`
	Summary     string           `json:"summary"`
	PublishedAt *time.Time       `json:"published_at"`
	Cover       *media.ImageFile `json:"cover"`
	CoverAlt    string           `json:"cover_alt"`
	IsSample    bool             `json:"is_sample"`
}

// Site reads the public snapshot. images may be nil (no object storage); news
// covers are then left out.
func (s *Service) Site(ctx context.Context, images *media.Images) (*Site, error) {
	q := dbq.New(s.pool)
	now := time.Now()
	site := &Site{GeneratedAt: now.UTC(), Settings: map[string]string{}}

	settings, err := q.PublicListSettings(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range settings {
		site.Settings[r.Key] = r.Value
	}

	banners, err := q.PublicListActiveBanners(ctx)
	if err != nil {
		return nil, err
	}
	site.Banners = make([]PublicBanner, 0, len(banners))
	for _, r := range banners {
		site.Banners = append(site.Banners, PublicBanner{Message: r.Message, LinkLabel: r.LinkLabel, LinkHref: r.LinkHref, EndsAt: r.EndsAt, IsSample: r.IsSample})
	}

	schools, err := q.PublicListSchools(ctx)
	if err != nil {
		return nil, err
	}
	site.Schools = make([]PublicSchool, 0, len(schools))
	for _, r := range schools {
		site.Schools = append(site.Schools, PublicSchool{Slug: r.Slug, Name: r.Name, ShortName: r.ShortName, AccentKey: r.AccentKey,
			Summary: r.Summary, Description: r.Description, HeadMessage: r.HeadMessage, IsSample: r.IsSample})
	}

	if site.Programmes, err = publicProgrammes(ctx, q, now); err != nil {
		return nil, err
	}
	slugs := map[uuid.UUID]string{}
	for _, p := range site.Programmes {
		slugs[p.id] = p.Slug
	}

	faqs, err := q.PublicListFaqs(ctx)
	if err != nil {
		return nil, err
	}
	site.FAQs = make([]PublicFAQ, 0, len(faqs))
	for _, r := range faqs {
		f := PublicFAQ{CategorySlug: r.CategorySlug, CategoryName: r.CategoryName, Question: r.Question, Answer: r.Answer, IsSample: r.IsSample}
		if r.ProgrammeID != nil {
			slug := slugs[*r.ProgrammeID]
			f.ProgrammeSlug = &slug
		}
		site.FAQs = append(site.FAQs, f)
	}

	stats, err := q.PublicListStatistics(ctx)
	if err != nil {
		return nil, err
	}
	site.Statistics = make([]PublicStatistic, 0, len(stats))
	for _, r := range stats {
		if r.VerifiedOn == nil {
			continue // cannot happen (table check), but never show an unverified figure
		}
		site.Statistics = append(site.Statistics, PublicStatistic{Label: r.Label, Value: r.Value, VerifiedOn: Date{*r.VerifiedOn}, IsSample: r.IsSample})
	}

	accs, err := q.PublicListAccreditations(ctx)
	if err != nil {
		return nil, err
	}
	site.Accreditations = make([]PublicAccreditation, 0, len(accs))
	for _, r := range accs {
		if r.VerifiedOn == nil {
			continue
		}
		site.Accreditations = append(site.Accreditations, PublicAccreditation{Body: r.Body, Reference: r.Reference,
			Description: r.Description, VerifiedOn: Date{*r.VerifiedOn}, IsSample: r.IsSample})
	}

	events, err := q.PublicListUpcomingEvents(ctx, homeListLimit)
	if err != nil {
		return nil, err
	}
	site.Events = make([]PublicEvent, 0, len(events))
	for _, r := range events {
		site.Events = append(site.Events, PublicEvent{Slug: r.Slug, Title: r.Title, Summary: r.Summary, Location: r.Location,
			StartsAt: r.StartsAt, EndsAt: r.EndsAt, IsSample: r.IsSample})
	}

	news, err := q.PublicListLatestNews(ctx, homeListLimit)
	if err != nil {
		return nil, err
	}
	site.News = make([]PublicNews, 0, len(news))
	for _, r := range news {
		n := PublicNews{Slug: r.Slug, Title: r.Title, Summary: r.Summary, PublishedAt: r.PublishedAt, IsSample: r.IsSample}
		if n.Cover = cover(ctx, images, r.CoverFileID); n.Cover != nil {
			n.CoverAlt = r.CoverAlt
		}
		site.News = append(site.News, n)
	}

	pages, err := q.PublicListPages(ctx)
	if err != nil {
		return nil, err
	}
	site.Pages = make([]PublicPage, 0, len(pages))
	for _, r := range pages {
		site.Pages = append(site.Pages, PublicPage{Slug: r.Slug, Title: r.Title, Summary: r.Summary, Body: r.Body,
			SEOTitle: r.SeoTitle, SEODescription: r.SeoDescription, PublishedAt: r.PublishedAt, UpdatedAt: r.UpdatedAt, IsSample: r.IsSample})
	}

	photos, err := q.PublicListGallery(ctx)
	if err != nil {
		return nil, err
	}
	site.Gallery = make([]PublicPhoto, 0, len(photos))
	for _, r := range photos {
		if images == nil {
			break
		}
		img, err := images.Get(ctx, r.FileID)
		if err != nil {
			continue // the file is gone or in another bucket; skip the photo
		}
		site.Gallery = append(site.Gallery, PublicPhoto{Image: img, Caption: r.Caption, AltText: r.AltText, IsSample: r.IsSample})
	}
	return site, nil
}

// cover looks up a news cover image; nil when there is none or storage is off.
func cover(ctx context.Context, images *media.Images, id *uuid.UUID) *media.ImageFile {
	if id == nil || images == nil {
		return nil
	}
	img, err := images.Get(ctx, *id)
	if err != nil {
		return nil
	}
	return img
}

// NewsPageSize is the number of articles per page of the news list.
const NewsPageSize = 12

type NewsList struct {
	Items      []PublicNews `json:"items"`
	Page       int          `json:"page"`
	TotalPages int          `json:"total_pages"`
	Total      int64        `json:"total"`
}

// NewsPage lists published articles, newest first. page starts at 1.
func (s *Service) NewsPage(ctx context.Context, images *media.Images, page int) (*NewsList, error) {
	q := dbq.New(s.pool)
	total, err := q.PublicCountNews(ctx)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	out := &NewsList{Items: []PublicNews{}, Page: page, Total: total, TotalPages: int((total + NewsPageSize - 1) / NewsPageSize)}
	rows, err := q.PublicListNewsPage(ctx, dbq.PublicListNewsPageParams{MaxItems: NewsPageSize, Skip: int32((page - 1) * NewsPageSize)})
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		n := PublicNews{Slug: r.Slug, Title: r.Title, Summary: r.Summary, PublishedAt: r.PublishedAt, IsSample: r.IsSample}
		if n.Cover = cover(ctx, images, r.CoverFileID); n.Cover != nil {
			n.CoverAlt = r.CoverAlt
		}
		out.Items = append(out.Items, n)
	}
	return out, nil
}

type NewsArticle struct {
	PublicNews
	Body           string    `json:"body"`
	SEOTitle       string    `json:"seo_title"`
	SEODescription string    `json:"seo_description"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (s *Service) NewsArticle(ctx context.Context, images *media.Images, slug string) (*NewsArticle, error) {
	r, err := dbq.New(s.pool).PublicGetNewsPost(ctx, slug)
	if err != nil {
		return nil, notFound(err)
	}
	a := &NewsArticle{
		PublicNews: PublicNews{Slug: r.Slug, Title: r.Title, Summary: r.Summary, PublishedAt: r.PublishedAt, IsSample: r.IsSample},
		Body:       r.Body, SEOTitle: r.SeoTitle, SEODescription: r.SeoDescription, UpdatedAt: r.UpdatedAt,
	}
	if a.Cover = cover(ctx, images, r.CoverFileID); a.Cover != nil {
		a.CoverAlt = r.CoverAlt
	}
	return a, nil
}

// pastEventsLimit caps the "recent events" list.
const pastEventsLimit = 12

type EventList struct {
	Upcoming []PublicEvent `json:"upcoming"`
	Past     []PublicEvent `json:"past"`
}

func (s *Service) Events(ctx context.Context) (*EventList, error) {
	q := dbq.New(s.pool)
	up, err := q.PublicListAllUpcomingEvents(ctx)
	if err != nil {
		return nil, err
	}
	past, err := q.PublicListPastEvents(ctx, pastEventsLimit)
	if err != nil {
		return nil, err
	}
	out := &EventList{Upcoming: make([]PublicEvent, 0, len(up)), Past: make([]PublicEvent, 0, len(past))}
	for _, r := range up {
		out.Upcoming = append(out.Upcoming, PublicEvent{Slug: r.Slug, Title: r.Title, Summary: r.Summary, Location: r.Location,
			StartsAt: r.StartsAt, EndsAt: r.EndsAt, IsSample: r.IsSample})
	}
	for _, r := range past {
		out.Past = append(out.Past, PublicEvent{Slug: r.Slug, Title: r.Title, Summary: r.Summary, Location: r.Location,
			StartsAt: r.StartsAt, EndsAt: r.EndsAt, IsSample: r.IsSample})
	}
	return out, nil
}

type EventDetail struct {
	PublicEvent
	Body        string     `json:"body"`
	PublishedAt *time.Time `json:"published_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (s *Service) Event(ctx context.Context, slug string) (*EventDetail, error) {
	r, err := dbq.New(s.pool).PublicGetEvent(ctx, slug)
	if err != nil {
		return nil, notFound(err)
	}
	return &EventDetail{
		PublicEvent: PublicEvent{Slug: r.Slug, Title: r.Title, Summary: r.Summary, Location: r.Location, StartsAt: r.StartsAt,
			EndsAt: r.EndsAt, IsSample: r.IsSample},
		Body: r.Body, PublishedAt: r.PublishedAt, UpdatedAt: r.UpdatedAt,
	}, nil
}

func publicProgrammes(ctx context.Context, q *dbq.Queries, now time.Time) ([]PublicProgramme, error) {
	rows, err := q.PublicListProgrammes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PublicProgramme, 0, len(rows))
	index := map[uuid.UUID]int{}
	for i, r := range rows {
		index[r.ID] = i
		out = append(out, PublicProgramme{
			Slug: r.Slug, Title: r.Title, SchoolSlug: r.SchoolSlug, Level: r.Level, Mode: r.Mode, DurationMonths: r.DurationMonths,
			DurationNote: r.DurationNote, MinKCSEGrade: r.MinKcseGrade, Summary: r.Summary, Overview: r.Overview,
			WhatYouLearn: r.WhatYouLearn, CareerOutcomes: r.CareerOutcomes, SEOTitle: r.SeoTitle, SEODescription: r.SeoDescription,
			Requirements: []Requirement{}, Intakes: []PublicIntake{}, Fees: []PublicFee{},
			PublishedAt: r.PublishedAt, UpdatedAt: r.UpdatedAt, IsSample: r.IsSample, id: r.ID,
		})
	}

	reqs, err := q.PublicListRequirements(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range reqs {
		if i, ok := index[r.ProgrammeID]; ok {
			out[i].Requirements = append(out[i].Requirements, Requirement{Kind: r.Kind, Requirement: r.Requirement})
		}
	}

	today := time.Date(now.In(nairobi).Year(), now.In(nairobi).Month(), now.In(nairobi).Day(), 0, 0, 0, 0, time.UTC)
	intakes, err := q.PublicListUpcomingIntakes(ctx, today)
	if err != nil {
		return nil, err
	}
	for _, r := range intakes {
		if i, ok := index[r.ProgrammeID]; ok {
			out[i].Intakes = append(out[i].Intakes, PublicIntake{ID: r.ID, Name: r.Name, StartsOn: Date{r.StartsOn},
				ApplicationsOpenOn: datePtr(r.ApplicationsOpenOn), ApplicationDeadline: datePtr(r.ApplicationDeadline), Status: r.Status})
		}
	}

	fees, err := q.PublicListFees(ctx)
	if err != nil {
		return nil, err
	}
	for _, r := range fees {
		if i, ok := index[r.ProgrammeID]; ok {
			out[i].Fees = append(out[i].Fees, PublicFee{IntakeID: r.IntakeID, Label: r.Label, Category: r.Category,
				AmountCents: r.AmountCents, Frequency: r.Frequency, Notes: r.Notes})
		}
	}
	return out, nil
}
