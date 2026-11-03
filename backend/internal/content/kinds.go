package content

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Kinds are the content types the staff portal edits, by URL name.
var Kinds = map[string]kind{}

func register[D Document](k def[D]) { Kinds[k.name] = k }

func init() {
	register(schoolKind)
	register(programmeKind)
	register(newsKind)
	register(eventKind)
	register(faqCategoryKind)
	register(faqKind)
	register(bannerKind)
	register(galleryKind)
	register(statisticKind)
	register(accreditationKind)
	register(pageKind)
	register(founderKind)
}

// KindNames lists the registered content types (for docs and tests).
func KindNames() []string {
	return []string{"schools", "programmes", "news", "events", "faq-categories", "faqs", "banners", "gallery", "statistics", "accreditations", "pages", "founders"}
}

func summaries[T any](rows []T, err error, f func(T) Summary) ([]Summary, error) {
	if err != nil {
		return nil, err
	}
	out := make([]Summary, 0, len(rows))
	for _, r := range rows {
		out = append(out, f(r))
	}
	return out, nil
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// ---------- schools ----------

type SchoolDoc struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	ShortName   string `json:"short_name"`
	AccentKey   string `json:"accent_key"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	HeadMessage string `json:"head_message"`
	SortOrder   int32  `json:"sort_order"`
	// PhotoFileID is the photo behind the school's card and page header.
	PhotoFileID           *uuid.UUID `json:"photo_file_id"`
	PhotoAlt              string     `json:"photo_alt"`
	PhotoConsentConfirmed bool       `json:"photo_consent_confirmed"`
}

func (d SchoolDoc) Validate() error {
	var p Problems
	p.slug("slug", d.Slug)
	p.required("name", d.Name, 120)
	p.required("short_name", d.ShortName, 60)
	p.oneOf("accent_key", d.AccentKey, "medical", "hospitality", "ict", "business", "pool")
	p.optional("summary", d.Summary, 400)
	p.optional("description", d.Description, 20000)
	p.optional("head_message", d.HeadMessage, 5000)
	photo(&p, d.PhotoFileID, d.PhotoAlt)
	return p.Err()
}

// photo checks an optional photo has a description for screen readers.
func photo(p *Problems, id *uuid.UUID, alt string) {
	if id != nil {
		p.required("photo_alt", alt, 250)
	} else {
		p.optional("photo_alt", alt, 250)
	}
}

// photoConsentMessage is the publishing rule for photos that may show people (NN-08).
const photoConsentMessage = "Confirm you have written consent from everyone recognisable in the photo before publishing."

var schoolKind = def[SchoolDoc]{
	name: "schools", workflow: true,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListSchools(ctx)
		return summaries(rows, err, func(r dbq.StaffListSchoolsRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}
		})
	},
	load: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (SchoolDoc, State, error) {
		r, err := q.StaffGetSchool(ctx, id)
		if err != nil {
			return SchoolDoc{}, State{}, notFound(err)
		}
		return SchoolDoc{Slug: r.Slug, Name: r.Name, ShortName: r.ShortName, AccentKey: r.AccentKey, Summary: r.Summary,
				Description: r.Description, HeadMessage: r.HeadMessage, SortOrder: r.SortOrder,
				PhotoFileID: r.PhotoFileID, PhotoAlt: r.PhotoAlt, PhotoConsentConfirmed: r.PhotoConsentConfirmed},
			State{Status: r.Status, PublishedAt: r.PublishedAt, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}, nil
	},
	create: func(ctx context.Context, q *dbq.Queries, d SchoolDoc, actor uuid.UUID) (uuid.UUID, error) {
		return q.StaffInsertSchool(ctx, dbq.StaffInsertSchoolParams{Slug: d.Slug, Name: d.Name, ShortName: d.ShortName,
			AccentKey: d.AccentKey, Summary: d.Summary, Description: d.Description, HeadMessage: d.HeadMessage, SortOrder: d.SortOrder,
			PhotoFileID: d.PhotoFileID, PhotoAlt: d.PhotoAlt, PhotoConsentConfirmed: d.PhotoConsentConfirmed, Actor: &actor})
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d SchoolDoc, actor uuid.UUID) error {
		return q.StaffUpdateSchool(ctx, dbq.StaffUpdateSchoolParams{ID: id, Slug: d.Slug, Name: d.Name, ShortName: d.ShortName,
			AccentKey: d.AccentKey, Summary: d.Summary, Description: d.Description, HeadMessage: d.HeadMessage, SortOrder: d.SortOrder,
			PhotoFileID: d.PhotoFileID, PhotoAlt: d.PhotoAlt, PhotoConsentConfirmed: d.PhotoConsentConfirmed, Actor: &actor})
	},
	setStatus: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, at *time.Time, actor uuid.UUID) error {
		return q.StaffSetSchoolStatus(ctx, dbq.StaffSetSchoolStatusParams{ID: id, Status: status, PublishedAt: at, Actor: &actor})
	},
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error { return q.StaffDeleteSchool(ctx, id) },
	readyToPublish: func(_ context.Context, _ *dbq.Queries, d SchoolDoc) error {
		var p Problems
		if d.Summary == "" {
			p.add("summary", "Add a short summary before publishing: it appears on the schools page.")
		}
		if d.PhotoFileID != nil && !d.PhotoConsentConfirmed {
			p.add("photo_consent_confirmed", photoConsentMessage)
		}
		return p.Err()
	},
}

// ---------- news ----------

type NewsDoc struct {
	Slug           string     `json:"slug"`
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	Body           string     `json:"body"`
	CoverFileID    *uuid.UUID `json:"cover_file_id"`
	CoverAlt       string     `json:"cover_alt"`
	SEOTitle       string     `json:"seo_title"`
	SEODescription string     `json:"seo_description"`
}

func (d NewsDoc) Validate() error {
	var p Problems
	p.slug("slug", d.Slug)
	p.required("title", d.Title, 160)
	p.optional("summary", d.Summary, 400)
	p.optional("body", d.Body, 50000)
	if d.CoverFileID != nil {
		p.required("cover_alt", d.CoverAlt, 250)
	}
	p.optional("seo_title", d.SEOTitle, 70)
	p.optional("seo_description", d.SEODescription, 170)
	return p.Err()
}

var newsKind = def[NewsDoc]{
	name: "news", workflow: true,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListNewsPosts(ctx)
		return summaries(rows, err, func(r dbq.StaffListNewsPostsRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}
		})
	},
	load: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (NewsDoc, State, error) {
		r, err := q.StaffGetNewsPost(ctx, id)
		if err != nil {
			return NewsDoc{}, State{}, notFound(err)
		}
		return NewsDoc{Slug: r.Slug, Title: r.Title, Summary: r.Summary, Body: r.Body, CoverFileID: r.CoverFileID, CoverAlt: r.CoverAlt,
				SEOTitle: r.SeoTitle, SEODescription: r.SeoDescription},
			State{Status: r.Status, PublishedAt: r.PublishedAt, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}, nil
	},
	create: func(ctx context.Context, q *dbq.Queries, d NewsDoc, actor uuid.UUID) (uuid.UUID, error) {
		return q.StaffInsertNewsPost(ctx, dbq.StaffInsertNewsPostParams{Slug: d.Slug, Title: d.Title, Summary: d.Summary, Body: d.Body,
			CoverFileID: d.CoverFileID, CoverAlt: d.CoverAlt, SeoTitle: d.SEOTitle, SeoDescription: d.SEODescription, Actor: &actor})
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d NewsDoc, _ uuid.UUID) error {
		return q.StaffUpdateNewsPost(ctx, dbq.StaffUpdateNewsPostParams{ID: id, Slug: d.Slug, Title: d.Title, Summary: d.Summary, Body: d.Body,
			CoverFileID: d.CoverFileID, CoverAlt: d.CoverAlt, SeoTitle: d.SEOTitle, SeoDescription: d.SEODescription})
	},
	setStatus: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, at *time.Time, _ uuid.UUID) error {
		return q.StaffSetNewsPostStatus(ctx, dbq.StaffSetNewsPostStatusParams{ID: id, Status: status, PublishedAt: at})
	},
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error { return q.StaffDeleteNewsPost(ctx, id) },
	readyToPublish: func(_ context.Context, _ *dbq.Queries, d NewsDoc) error {
		var p Problems
		if d.Summary == "" {
			p.add("summary", "Add a summary before publishing: it shows in the news list.")
		}
		if d.Body == "" {
			p.add("body", "Write the article before publishing.")
		}
		return p.Err()
	},
}

// ---------- pages ----------

// PageDoc is a text page such as student life or the privacy notice. Body is
// plain text: blank lines separate paragraphs, "## " starts a heading and
// "- " a list item (rendered by the website, never as HTML).
type PageDoc struct {
	Slug           string `json:"slug"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	Body           string `json:"body"`
	SEOTitle       string `json:"seo_title"`
	SEODescription string `json:"seo_description"`
}

func (d PageDoc) Validate() error {
	var p Problems
	p.slug("slug", d.Slug)
	p.required("title", d.Title, 160)
	p.optional("summary", d.Summary, 400)
	p.optional("body", d.Body, 100000)
	p.optional("seo_title", d.SEOTitle, 70)
	p.optional("seo_description", d.SEODescription, 170)
	return p.Err()
}

var pageKind = def[PageDoc]{
	name: "pages", workflow: true,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListPages(ctx)
		return summaries(rows, err, func(r dbq.StaffListPagesRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Context: "/" + r.Slug, Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}
		})
	},
	load: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (PageDoc, State, error) {
		r, err := q.StaffGetPage(ctx, id)
		if err != nil {
			return PageDoc{}, State{}, notFound(err)
		}
		return PageDoc{Slug: r.Slug, Title: r.Title, Summary: r.Summary, Body: r.Body, SEOTitle: r.SeoTitle, SEODescription: r.SeoDescription},
			State{Status: r.Status, PublishedAt: r.PublishedAt, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}, nil
	},
	create: func(ctx context.Context, q *dbq.Queries, d PageDoc, actor uuid.UUID) (uuid.UUID, error) {
		return q.StaffInsertPage(ctx, dbq.StaffInsertPageParams{Slug: d.Slug, Title: d.Title, Summary: d.Summary, Body: d.Body,
			SeoTitle: d.SEOTitle, SeoDescription: d.SEODescription, Actor: &actor})
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d PageDoc, actor uuid.UUID) error {
		return q.StaffUpdatePage(ctx, dbq.StaffUpdatePageParams{ID: id, Slug: d.Slug, Title: d.Title, Summary: d.Summary, Body: d.Body,
			SeoTitle: d.SEOTitle, SeoDescription: d.SEODescription, Actor: &actor})
	},
	setStatus: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, at *time.Time, actor uuid.UUID) error {
		return q.StaffSetPageStatus(ctx, dbq.StaffSetPageStatusParams{ID: id, Status: status, PublishedAt: at, Actor: &actor})
	},
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error { return q.StaffDeletePage(ctx, id) },
	readyToPublish: func(_ context.Context, _ *dbq.Queries, d PageDoc) error {
		var p Problems
		if d.Body == "" {
			p.add("body", "Write the page before publishing.")
		}
		return p.Err()
	},
}

// ---------- events ----------

type EventDoc struct {
	Slug     string     `json:"slug"`
	Title    string     `json:"title"`
	Summary  string     `json:"summary"`
	Body     string     `json:"body"`
	Location string     `json:"location"`
	StartsAt time.Time  `json:"starts_at"`
	EndsAt   *time.Time `json:"ends_at"`
}

func (d EventDoc) Validate() error {
	var p Problems
	p.slug("slug", d.Slug)
	p.required("title", d.Title, 160)
	p.optional("summary", d.Summary, 400)
	p.optional("body", d.Body, 20000)
	p.optional("location", d.Location, 200)
	if d.StartsAt.IsZero() {
		p.add("starts_at", "Choose when the event starts.")
	}
	if d.EndsAt != nil && d.EndsAt.Before(d.StartsAt) {
		p.add("ends_at", "The end must be after the start.")
	}
	return p.Err()
}

var eventKind = def[EventDoc]{
	name: "events", workflow: true,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListEvents(ctx)
		return summaries(rows, err, func(r dbq.StaffListEventsRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}
		})
	},
	load: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (EventDoc, State, error) {
		r, err := q.StaffGetEvent(ctx, id)
		if err != nil {
			return EventDoc{}, State{}, notFound(err)
		}
		return EventDoc{Slug: r.Slug, Title: r.Title, Summary: r.Summary, Body: r.Body, Location: r.Location, StartsAt: r.StartsAt, EndsAt: r.EndsAt},
			State{Status: r.Status, PublishedAt: r.PublishedAt, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}, nil
	},
	create: func(ctx context.Context, q *dbq.Queries, d EventDoc, _ uuid.UUID) (uuid.UUID, error) {
		return q.StaffInsertEvent(ctx, dbq.StaffInsertEventParams{Slug: d.Slug, Title: d.Title, Summary: d.Summary, Body: d.Body,
			Location: d.Location, StartsAt: d.StartsAt, EndsAt: d.EndsAt})
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d EventDoc, _ uuid.UUID) error {
		return q.StaffUpdateEvent(ctx, dbq.StaffUpdateEventParams{ID: id, Slug: d.Slug, Title: d.Title, Summary: d.Summary, Body: d.Body,
			Location: d.Location, StartsAt: d.StartsAt, EndsAt: d.EndsAt})
	},
	setStatus: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, at *time.Time, _ uuid.UUID) error {
		return q.StaffSetEventStatus(ctx, dbq.StaffSetEventStatusParams{ID: id, Status: status, PublishedAt: at})
	},
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error { return q.StaffDeleteEvent(ctx, id) },
}

// ---------- FAQ categories (no workflow) ----------

type FaqCategoryDoc struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	SortOrder int32  `json:"sort_order"`
}

func (d FaqCategoryDoc) Validate() error {
	var p Problems
	p.slug("slug", d.Slug)
	p.required("name", d.Name, 80)
	return p.Err()
}

var faqCategoryKind = def[FaqCategoryDoc]{
	name: "faq-categories", workflow: false,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListFaqCategories(ctx)
		return summaries(rows, err, func(r dbq.StaffListFaqCategoriesRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Status: "published", IsSample: r.IsSample}
		})
	},
	load: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (FaqCategoryDoc, State, error) {
		r, err := q.StaffGetFaqCategory(ctx, id)
		if err != nil {
			return FaqCategoryDoc{}, State{}, notFound(err)
		}
		return FaqCategoryDoc{Slug: r.Slug, Name: r.Name, SortOrder: r.SortOrder}, State{Status: "published", IsSample: r.IsSample}, nil
	},
	create: func(ctx context.Context, q *dbq.Queries, d FaqCategoryDoc, _ uuid.UUID) (uuid.UUID, error) {
		return q.StaffInsertFaqCategory(ctx, dbq.StaffInsertFaqCategoryParams{Slug: d.Slug, Name: d.Name, SortOrder: d.SortOrder})
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d FaqCategoryDoc, _ uuid.UUID) error {
		return q.StaffUpdateFaqCategory(ctx, dbq.StaffUpdateFaqCategoryParams{ID: id, Slug: d.Slug, Name: d.Name, SortOrder: d.SortOrder})
	},
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error {
		return q.StaffDeleteFaqCategory(ctx, id)
	},
}

// ---------- FAQs ----------

type FaqDoc struct {
	CategoryID  uuid.UUID  `json:"category_id"`
	ProgrammeID *uuid.UUID `json:"programme_id"`
	Question    string     `json:"question"`
	Answer      string     `json:"answer"`
	SortOrder   int32      `json:"sort_order"`
}

func (d FaqDoc) Validate() error {
	var p Problems
	if d.CategoryID == uuid.Nil {
		p.add("category_id", "Choose a category.")
	}
	p.required("question", d.Question, 300)
	p.required("answer", d.Answer, 5000)
	return p.Err()
}

var faqKind = def[FaqDoc]{
	name: "faqs", workflow: true,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListFaqs(ctx)
		return summaries(rows, err, func(r dbq.StaffListFaqsRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Context: r.CategoryName, Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}
		})
	},
	load: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (FaqDoc, State, error) {
		r, err := q.StaffGetFaq(ctx, id)
		if err != nil {
			return FaqDoc{}, State{}, notFound(err)
		}
		return FaqDoc{CategoryID: r.CategoryID, ProgrammeID: r.ProgrammeID, Question: r.Question, Answer: r.Answer, SortOrder: r.SortOrder},
			State{Status: r.Status, PublishedAt: r.PublishedAt, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}, nil
	},
	create: func(ctx context.Context, q *dbq.Queries, d FaqDoc, _ uuid.UUID) (uuid.UUID, error) {
		return q.StaffInsertFaq(ctx, dbq.StaffInsertFaqParams{CategoryID: d.CategoryID, ProgrammeID: d.ProgrammeID, Question: d.Question, Answer: d.Answer, SortOrder: d.SortOrder})
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d FaqDoc, _ uuid.UUID) error {
		return q.StaffUpdateFaq(ctx, dbq.StaffUpdateFaqParams{ID: id, CategoryID: d.CategoryID, ProgrammeID: d.ProgrammeID, Question: d.Question, Answer: d.Answer, SortOrder: d.SortOrder})
	},
	setStatus: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, at *time.Time, _ uuid.UUID) error {
		return q.StaffSetFaqStatus(ctx, dbq.StaffSetFaqStatusParams{ID: id, Status: status, PublishedAt: at})
	},
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error { return q.StaffDeleteFaq(ctx, id) },
}

// ---------- banners (announcement bar) ----------

type BannerDoc struct {
	Message   string    `json:"message"`
	LinkLabel *string   `json:"link_label"`
	LinkHref  *string   `json:"link_href"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
}

func (d BannerDoc) Validate() error {
	var p Problems
	p.required("message", d.Message, 200)
	if (d.LinkLabel == nil) != (d.LinkHref == nil) {
		p.add("link_href", "Give both a link label and a link, or neither.")
	}
	if d.LinkLabel != nil {
		p.required("link_label", *d.LinkLabel, 40)
		p.link("link_href", *d.LinkHref)
	}
	if d.StartsAt.IsZero() || d.EndsAt.IsZero() {
		p.add("ends_at", "Choose when the announcement starts and ends.")
	} else if !d.EndsAt.After(d.StartsAt) {
		p.add("ends_at", "The end must be after the start.")
	}
	return p.Err()
}

var bannerKind = def[BannerDoc]{
	name: "banners", workflow: true,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListBanners(ctx)
		return summaries(rows, err, func(r dbq.StaffListBannersRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}
		})
	},
	load: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (BannerDoc, State, error) {
		r, err := q.StaffGetBanner(ctx, id)
		if err != nil {
			return BannerDoc{}, State{}, notFound(err)
		}
		return BannerDoc{Message: r.Message, LinkLabel: r.LinkLabel, LinkHref: r.LinkHref, StartsAt: r.StartsAt, EndsAt: r.EndsAt},
			State{Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}, nil
	},
	create: func(ctx context.Context, q *dbq.Queries, d BannerDoc, _ uuid.UUID) (uuid.UUID, error) {
		return q.StaffInsertBanner(ctx, dbq.StaffInsertBannerParams{Message: d.Message, LinkLabel: d.LinkLabel, LinkHref: d.LinkHref, StartsAt: d.StartsAt, EndsAt: d.EndsAt})
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d BannerDoc, _ uuid.UUID) error {
		return q.StaffUpdateBanner(ctx, dbq.StaffUpdateBannerParams{ID: id, Message: d.Message, LinkLabel: d.LinkLabel, LinkHref: d.LinkHref, StartsAt: d.StartsAt, EndsAt: d.EndsAt})
	},
	setStatus: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, _ *time.Time, _ uuid.UUID) error {
		return q.StaffSetBannerStatus(ctx, dbq.StaffSetBannerStatusParams{ID: id, Status: status})
	},
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error { return q.StaffDeleteBanner(ctx, id) },
}

// ---------- gallery ----------

type GalleryDoc struct {
	FileID           uuid.UUID `json:"file_id"`
	Caption          string    `json:"caption"`
	AltText          string    `json:"alt_text"`
	ConsentConfirmed bool      `json:"consent_confirmed"`
	ConsentNote      string    `json:"consent_note"`
	// HomeHero puts the photo at the top of the home page.
	HomeHero bool `json:"home_hero"`
	// Alumni shows the photo on the Alumni page.
	Alumni    bool  `json:"alumni"`
	SortOrder int32 `json:"sort_order"`
}

func (d GalleryDoc) Validate() error {
	var p Problems
	if d.FileID == uuid.Nil {
		p.add("file_id", "Upload a photo.")
	}
	p.required("alt_text", d.AltText, 250)
	p.optional("caption", d.Caption, 300)
	p.optional("consent_note", d.ConsentNote, 1000)
	return p.Err()
}

var galleryKind = def[GalleryDoc]{
	name: "gallery", workflow: true,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListGalleryItems(ctx)
		return summaries(rows, err, func(r dbq.StaffListGalleryItemsRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}
		})
	},
	load: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (GalleryDoc, State, error) {
		r, err := q.StaffGetGalleryItem(ctx, id)
		if err != nil {
			return GalleryDoc{}, State{}, notFound(err)
		}
		return GalleryDoc{FileID: r.FileID, Caption: r.Caption, AltText: r.AltText, ConsentConfirmed: r.ConsentConfirmed, ConsentNote: r.ConsentNote, HomeHero: r.HomeHero, Alumni: r.Alumni, SortOrder: r.SortOrder},
			State{Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}, nil
	},
	create: func(ctx context.Context, q *dbq.Queries, d GalleryDoc, _ uuid.UUID) (uuid.UUID, error) {
		return q.StaffInsertGalleryItem(ctx, dbq.StaffInsertGalleryItemParams{FileID: d.FileID, Caption: d.Caption, AltText: d.AltText,
			ConsentConfirmed: d.ConsentConfirmed, ConsentNote: d.ConsentNote, HomeHero: d.HomeHero, Alumni: d.Alumni, SortOrder: d.SortOrder})
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d GalleryDoc, _ uuid.UUID) error {
		return q.StaffUpdateGalleryItem(ctx, dbq.StaffUpdateGalleryItemParams{ID: id, FileID: d.FileID, Caption: d.Caption, AltText: d.AltText,
			ConsentConfirmed: d.ConsentConfirmed, ConsentNote: d.ConsentNote, HomeHero: d.HomeHero, Alumni: d.Alumni, SortOrder: d.SortOrder})
	},
	setStatus: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, _ *time.Time, _ uuid.UUID) error {
		return q.StaffSetGalleryItemStatus(ctx, dbq.StaffSetGalleryItemStatusParams{ID: id, Status: status})
	},
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error {
		return q.StaffDeleteGalleryItem(ctx, id)
	},
	readyToPublish: func(_ context.Context, _ *dbq.Queries, d GalleryDoc) error {
		if !d.ConsentConfirmed {
			return Problems{{Field: "consent_confirmed", Message: photoConsentMessage}}.Err()
		}
		return nil
	},
}

// ---------- founders ----------

// FounderDoc is one founder's profile. Story follows the page text rules.
type FounderDoc struct {
	Name             string     `json:"name"`
	RoleTitle        string     `json:"role_title"`
	Intro            string     `json:"intro"`
	Story            string     `json:"story"`
	PhotoFileID      *uuid.UUID `json:"photo_file_id"`
	PhotoAlt         string     `json:"photo_alt"`
	ConsentConfirmed bool       `json:"consent_confirmed"`
	ConsentNote      string     `json:"consent_note"`
	SortOrder        int32      `json:"sort_order"`
}

func (d FounderDoc) Validate() error {
	var p Problems
	p.required("name", d.Name, 120)
	p.optional("role_title", d.RoleTitle, 120)
	p.optional("intro", d.Intro, 300)
	p.optional("story", d.Story, 20000)
	photo(&p, d.PhotoFileID, d.PhotoAlt)
	p.optional("consent_note", d.ConsentNote, 1000)
	return p.Err()
}

var founderKind = def[FounderDoc]{
	name: "founders", workflow: true,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListFounders(ctx)
		return summaries(rows, err, func(r dbq.StaffListFoundersRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Context: r.RoleTitle, Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}
		})
	},
	load: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (FounderDoc, State, error) {
		r, err := q.StaffGetFounder(ctx, id)
		if err != nil {
			return FounderDoc{}, State{}, notFound(err)
		}
		return FounderDoc{Name: r.Name, RoleTitle: r.RoleTitle, Intro: r.Intro, Story: r.Story, PhotoFileID: r.PhotoFileID,
				PhotoAlt: r.PhotoAlt, ConsentConfirmed: r.ConsentConfirmed, ConsentNote: r.ConsentNote, SortOrder: r.SortOrder},
			State{Status: r.Status, PublishedAt: r.PublishedAt, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}, nil
	},
	create: func(ctx context.Context, q *dbq.Queries, d FounderDoc, actor uuid.UUID) (uuid.UUID, error) {
		return q.StaffInsertFounder(ctx, dbq.StaffInsertFounderParams{Name: d.Name, RoleTitle: d.RoleTitle, Intro: d.Intro, Story: d.Story,
			PhotoFileID: d.PhotoFileID, PhotoAlt: d.PhotoAlt, ConsentConfirmed: d.ConsentConfirmed, ConsentNote: d.ConsentNote,
			SortOrder: d.SortOrder, Actor: &actor})
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d FounderDoc, actor uuid.UUID) error {
		return q.StaffUpdateFounder(ctx, dbq.StaffUpdateFounderParams{ID: id, Name: d.Name, RoleTitle: d.RoleTitle, Intro: d.Intro, Story: d.Story,
			PhotoFileID: d.PhotoFileID, PhotoAlt: d.PhotoAlt, ConsentConfirmed: d.ConsentConfirmed, ConsentNote: d.ConsentNote,
			SortOrder: d.SortOrder, Actor: &actor})
	},
	setStatus: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, at *time.Time, actor uuid.UUID) error {
		return q.StaffSetFounderStatus(ctx, dbq.StaffSetFounderStatusParams{ID: id, Status: status, PublishedAt: at, Actor: &actor})
	},
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error { return q.StaffDeleteFounder(ctx, id) },
	readyToPublish: func(_ context.Context, _ *dbq.Queries, d FounderDoc) error {
		var p Problems
		if d.Story == "" {
			p.add("story", "Write the founder's story before publishing.")
		}
		if !d.ConsentConfirmed {
			p.add("consent_confirmed", "Confirm the founder has agreed in writing to their profile and photo being published.")
		}
		return p.Err()
	},
}

// ---------- statistics and accreditations (evidence required, NN-01) ----------

type StatisticDoc struct {
	Label      string `json:"label"`
	Value      string `json:"value"`
	SourceNote string `json:"source_note"`
	VerifiedOn *Date  `json:"verified_on"`
	SortOrder  int32  `json:"sort_order"`
}

func (d StatisticDoc) Validate() error {
	var p Problems
	p.required("label", d.Label, 80)
	p.required("value", d.Value, 30)
	p.optional("source_note", d.SourceNote, 1000)
	if d.VerifiedOn != nil && d.VerifiedOn.After(time.Now()) {
		p.add("verified_on", "The verification date cannot be in the future.")
	}
	return p.Err()
}

func evidenceReady(verified *Date, evidence, evidenceField string) error {
	var p Problems
	if verified == nil {
		p.add("verified_on", "Enter the date this was checked against evidence. Unverified figures are never shown.")
	}
	if evidence == "" {
		p.add(evidenceField, "Say where the evidence is, for example the report or certificate it comes from.")
	}
	return p.Err()
}

func verifiedBy(verified *Date, actor uuid.UUID) *uuid.UUID {
	if verified == nil {
		return nil
	}
	return &actor
}

var statisticKind = def[StatisticDoc]{
	name: "statistics", workflow: true,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListStatistics(ctx)
		return summaries(rows, err, func(r dbq.StaffListStatisticsRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}
		})
	},
	load: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (StatisticDoc, State, error) {
		r, err := q.StaffGetStatistic(ctx, id)
		if err != nil {
			return StatisticDoc{}, State{}, notFound(err)
		}
		return StatisticDoc{Label: r.Label, Value: r.Value, SourceNote: r.SourceNote, VerifiedOn: datePtr(r.VerifiedOn), SortOrder: r.SortOrder},
			State{Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}, nil
	},
	create: func(ctx context.Context, q *dbq.Queries, d StatisticDoc, actor uuid.UUID) (uuid.UUID, error) {
		return q.StaffInsertStatistic(ctx, dbq.StaffInsertStatisticParams{Label: d.Label, Value: d.Value, SourceNote: d.SourceNote,
			VerifiedOn: timePtr(d.VerifiedOn), VerifiedBy: verifiedBy(d.VerifiedOn, actor), SortOrder: d.SortOrder})
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d StatisticDoc, actor uuid.UUID) error {
		return q.StaffUpdateStatistic(ctx, dbq.StaffUpdateStatisticParams{ID: id, Label: d.Label, Value: d.Value, SourceNote: d.SourceNote,
			VerifiedOn: timePtr(d.VerifiedOn), VerifiedBy: verifiedBy(d.VerifiedOn, actor), SortOrder: d.SortOrder})
	},
	setStatus: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, _ *time.Time, _ uuid.UUID) error {
		return q.StaffSetStatisticStatus(ctx, dbq.StaffSetStatisticStatusParams{ID: id, Status: status})
	},
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error { return q.StaffDeleteStatistic(ctx, id) },
	readyToPublish: func(_ context.Context, _ *dbq.Queries, d StatisticDoc) error {
		return evidenceReady(d.VerifiedOn, d.SourceNote, "source_note")
	},
}

type AccreditationDoc struct {
	Body         string `json:"body"`
	Reference    string `json:"reference"`
	Description  string `json:"description"`
	EvidenceNote string `json:"evidence_note"`
	VerifiedOn   *Date  `json:"verified_on"`
	SortOrder    int32  `json:"sort_order"`
}

func (d AccreditationDoc) Validate() error {
	var p Problems
	p.required("body", d.Body, 160)
	p.required("reference", d.Reference, 120)
	p.optional("description", d.Description, 2000)
	p.optional("evidence_note", d.EvidenceNote, 1000)
	if d.VerifiedOn != nil && d.VerifiedOn.After(time.Now()) {
		p.add("verified_on", "The verification date cannot be in the future.")
	}
	return p.Err()
}

var accreditationKind = def[AccreditationDoc]{
	name: "accreditations", workflow: true,
	list: func(ctx context.Context, q *dbq.Queries) ([]Summary, error) {
		rows, err := q.StaffListAccreditations(ctx)
		return summaries(rows, err, func(r dbq.StaffListAccreditationsRow) Summary {
			return Summary{ID: r.ID, Label: r.Label, Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}
		})
	},
	load: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (AccreditationDoc, State, error) {
		r, err := q.StaffGetAccreditation(ctx, id)
		if err != nil {
			return AccreditationDoc{}, State{}, notFound(err)
		}
		return AccreditationDoc{Body: r.Body, Reference: r.Reference, Description: r.Description, EvidenceNote: r.EvidenceNote,
				VerifiedOn: datePtr(r.VerifiedOn), SortOrder: r.SortOrder},
			State{Status: r.Status, IsSample: r.IsSample, UpdatedAt: r.UpdatedAt}, nil
	},
	create: func(ctx context.Context, q *dbq.Queries, d AccreditationDoc, actor uuid.UUID) (uuid.UUID, error) {
		return q.StaffInsertAccreditation(ctx, dbq.StaffInsertAccreditationParams{Body: d.Body, Reference: d.Reference, Description: d.Description,
			EvidenceNote: d.EvidenceNote, VerifiedOn: timePtr(d.VerifiedOn), VerifiedBy: verifiedBy(d.VerifiedOn, actor), SortOrder: d.SortOrder})
	},
	save: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d AccreditationDoc, actor uuid.UUID) error {
		return q.StaffUpdateAccreditation(ctx, dbq.StaffUpdateAccreditationParams{ID: id, Body: d.Body, Reference: d.Reference, Description: d.Description,
			EvidenceNote: d.EvidenceNote, VerifiedOn: timePtr(d.VerifiedOn), VerifiedBy: verifiedBy(d.VerifiedOn, actor), SortOrder: d.SortOrder})
	},
	setStatus: func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, _ *time.Time, _ uuid.UUID) error {
		return q.StaffSetAccreditationStatus(ctx, dbq.StaffSetAccreditationStatusParams{ID: id, Status: status})
	},
	remove: func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error {
		return q.StaffDeleteAccreditation(ctx, id)
	},
	readyToPublish: func(_ context.Context, _ *dbq.Queries, d AccreditationDoc) error {
		return evidenceReady(d.VerifiedOn, d.EvidenceNote, "evidence_note")
	},
}
