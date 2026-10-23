package application

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Steps of the form, in order. Missing items name the step they belong to.
const (
	StepChoice    = "choice"
	StepPersonal  = "personal"
	StepGuardian  = "guardian"
	StepEducation = "education"
	StepSponsor   = "sponsor"
	StepDocuments = "documents"
)

var (
	documentTypes  = []string{"national_id", "passport", "birth_certificate"}
	relationships  = []string{"mother", "father", "guardian", "other_relative", "other"}
	educationLevel = []string{"kcse", "kcpe", "certificate", "diploma", "degree", "other"}
	sponsorTypes   = []string{"self", "parent_guardian", "sponsor", "other"}
)

type Intake struct {
	ID                  uuid.UUID `json:"id"`
	Name                string    `json:"name"`
	StartsOn            string    `json:"starts_on"`
	ApplicationDeadline *string   `json:"application_deadline"`
}

type Personal struct {
	FirstName            string `json:"first_name"`
	MiddleName           string `json:"middle_name"`
	LastName             string `json:"last_name"`
	DateOfBirth          string `json:"date_of_birth"`
	IdentityDocumentType string `json:"identity_document_type"`
	IdentityDocument     string `json:"identity_document"`
	Nationality          string `json:"nationality"`
	County               string `json:"county"`
	Phone                string `json:"phone"`
}

type Guardian struct {
	FullName     string `json:"full_name"`
	Relationship string `json:"relationship"`
	Phone        string `json:"phone"`
	Email        string `json:"email"`
	// ConsentGiven: the applicant confirms that this parent or guardian has
	// read the privacy notice and agrees (recorded in consents, append-only).
	ConsentGiven bool `json:"consent_given"`
}

type SubjectGrade struct {
	Subject string `json:"subject"`
	Grade   string `json:"grade"`
}

type Education struct {
	Level         string         `json:"level"`
	Institution   string         `json:"institution"`
	YearCompleted *int           `json:"year_completed"`
	Qualification string         `json:"qualification"`
	MeanGrade     string         `json:"mean_grade"`
	Subjects      []SubjectGrade `json:"subjects"`
}

type Sponsor struct {
	Type  string `json:"type"`
	Name  string `json:"name"`
	Phone string `json:"phone"`
}

// Missing is one thing still needed before the application can be submitted.
type Missing struct {
	Step    string `json:"step"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Detail is the whole application as its owner sees it.
type Detail struct {
	ID             uuid.UUID   `json:"id"`
	Reference      string      `json:"reference"`
	Status         string      `json:"status"`
	Editable       bool        `json:"editable"`
	ProgrammeTitle string      `json:"programme_title"`
	ProgrammeSlug  string      `json:"programme_slug"`
	IntakeID       *uuid.UUID  `json:"intake_id"`
	IntakeName     *string     `json:"intake_name"`
	IntakeStartsOn *string     `json:"intake_starts_on"`
	OpenIntakes    []Intake    `json:"open_intakes"`
	Email          string      `json:"email"`
	Personal       Personal    `json:"personal"`
	NeedsGuardian  bool        `json:"needs_guardian"`
	Guardian       Guardian    `json:"guardian"`
	Education      []Education `json:"education"`
	Sponsor        Sponsor     `json:"sponsor"`
	Documents      []Document  `json:"documents"`
	// RequiredDocuments are the document kinds this application needs.
	RequiredDocuments []string     `json:"required_documents"`
	Missing           []Missing    `json:"missing"`
	History           []HistoryRow `json:"history"`
	DraftExpiresAt    *time.Time   `json:"draft_expires_at"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
	SubmittedAt       *time.Time   `json:"submitted_at"`
}

func dateString(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(time.DateOnly)
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Get returns the application if userID owns it.
func (s *Service) Get(ctx context.Context, userID, id uuid.UUID) (*Detail, error) {
	a, err := s.load(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	q := dbq.New(s.pool)
	ap, err := q.GetApplicant(ctx, a.ApplicantID)
	if err != nil {
		return nil, err
	}
	d := &Detail{
		ID: a.ID, Reference: a.Reference, Status: a.Status, Editable: a.Status == "draft",
		ProgrammeTitle: a.ProgrammeTitle, ProgrammeSlug: a.ProgrammeSlug,
		IntakeID: a.IntakeID, IntakeName: a.IntakeName, IntakeStartsOn: dateString(a.IntakeStartsOn),
		Email: ap.Email,
		Personal: Personal{
			FirstName: ap.FirstName, MiddleName: ap.MiddleName, LastName: ap.LastName,
			DateOfBirth: deref(dateString(ap.DateOfBirth)), IdentityDocumentType: deref(ap.IdentityDocumentType),
			IdentityDocument: ap.IdentityDocument, Nationality: ap.Nationality, County: ap.County, Phone: deref(ap.Phone),
		},
		Sponsor:        Sponsor{Type: deref(a.SponsorType), Name: a.SponsorName, Phone: a.SponsorPhone},
		Education:      []Education{},
		OpenIntakes:    []Intake{},
		DraftExpiresAt: a.DraftExpiresAt, CreatedAt: a.CreatedAt, UpdatedAt: a.UpdatedAt, SubmittedAt: a.SubmittedAt,
	}
	if !d.Editable {
		d.DraftExpiresAt = nil
	}
	d.NeedsGuardian = s.isMinor(ap.DateOfBirth)

	open, err := q.ListOpenIntakes(ctx, dbq.ListOpenIntakesParams{ProgrammeID: a.ProgrammeID, Today: s.today()})
	if err != nil {
		return nil, err
	}
	for _, i := range open {
		d.OpenIntakes = append(d.OpenIntakes, Intake{ID: i.ID, Name: i.Name, StartsOn: i.StartsOn.Format(time.DateOnly), ApplicationDeadline: dateString(i.ApplicationDeadline)})
	}

	g, err := q.GetGuardian(ctx, a.ID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return nil, err
	default:
		d.Guardian = Guardian{FullName: g.FullName, Relationship: g.Relationship, Phone: g.Phone, Email: deref(g.Email)}
		if d.Guardian.ConsentGiven, err = consentHolds(ctx, q, g.ID, g.FullName); err != nil {
			return nil, err
		}
	}

	rows, err := q.ListEducation(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	for _, r := range rows {
		e := Education{Level: r.Level, Institution: r.Institution, Qualification: r.Qualification, MeanGrade: r.MeanGrade, Subjects: []SubjectGrade{}}
		if r.YearCompleted != nil {
			y := int(*r.YearCompleted)
			e.YearCompleted = &y
		}
		if err := json.Unmarshal(r.SubjectGrades, &e.Subjects); err != nil {
			return nil, err
		}
		d.Education = append(d.Education, e)
	}

	docs, err := q.ListApplicationDocuments(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	d.Documents = make([]Document, 0, len(docs))
	for _, r := range docs {
		d.Documents = append(d.Documents, Document{ID: r.ID, Kind: r.Kind, Name: r.OriginalName, ContentType: r.ContentType,
			SizeBytes: r.SizeBytes, ScanStatus: r.ScanStatus, CreatedAt: r.CreatedAt})
	}
	d.RequiredDocuments = d.requiredDocuments()

	hist, err := q.ListStatusHistory(ctx, a.ID)
	if err != nil {
		return nil, err
	}
	d.History = make([]HistoryRow, 0, len(hist))
	for _, h := range hist {
		d.History = append(d.History, HistoryRow{Status: h.ToStatus, Reason: h.Reason, At: h.CreatedAt})
	}

	d.Missing = d.missing()
	return d, nil
}

// HistoryRow is one status change, as the applicant sees it.
type HistoryRow struct {
	Status string    `json:"status"`
	Reason string    `json:"reason"`
	At     time.Time `json:"at"`
}

// consentHolds is true when the latest guardian consent is given and was
// given in the name of the guardian now on the form.
func consentHolds(ctx context.Context, q *dbq.Queries, guardianID uuid.UUID, name string) (bool, error) {
	c, err := q.LatestConsent(ctx, dbq.LatestConsentParams{SubjectType: "guardian", SubjectID: guardianID, Purpose: "guardian_consent"})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return c.Given && c.GivenByName == name, nil
}

// isMinor: under AdultAge today (Nairobi). An unknown date of birth counts as
// not yet known, so the guardian step is shown only once it is.
func (s *Service) isMinor(dob *time.Time) bool {
	if dob == nil {
		return false
	}
	return ageOn(*dob, s.today()) < AdultAge
}

func ageOn(dob, day time.Time) int {
	age := day.Year() - dob.Year()
	if day.Month() < dob.Month() || (day.Month() == dob.Month() && day.Day() < dob.Day()) {
		age--
	}
	return age
}

// missing lists what the form still needs, step by step (brief 8). The same
// list blocks submission in Stage 9b.
func (d *Detail) missing() []Missing {
	out := []Missing{}
	add := func(step, field, msg string) { out = append(out, Missing{Step: step, Field: field, Message: msg}) }

	switch {
	case len(d.OpenIntakes) == 0:
		add(StepChoice, "intake_id", "This programme is not taking applications now. Contact the admissions office.")
	case d.IntakeID == nil || !slices.ContainsFunc(d.OpenIntakes, func(i Intake) bool { return i.ID == *d.IntakeID }):
		add(StepChoice, "intake_id", "Choose an intake that is taking applications.")
	}

	p := d.Personal
	for _, f := range []struct{ field, value, msg string }{
		{"first_name", p.FirstName, "Enter your first name."},
		{"last_name", p.LastName, "Enter your last name."},
		{"date_of_birth", p.DateOfBirth, "Enter your date of birth."},
		{"identity_document_type", p.IdentityDocumentType, "Choose your identity document."},
		{"identity_document", p.IdentityDocument, "Enter your identity document number."},
		{"phone", p.Phone, "Enter a phone number we can call you on."},
		{"nationality", p.Nationality, "Enter your nationality."},
		{"county", p.County, "Choose the county where you live."},
	} {
		if f.value == "" {
			add(StepPersonal, f.field, f.msg)
		}
	}

	if d.NeedsGuardian {
		g := d.Guardian
		if g.FullName == "" {
			add(StepGuardian, "full_name", "Enter your parent or guardian's name.")
		}
		if g.Relationship == "" {
			add(StepGuardian, "relationship", "Choose how they are related to you.")
		}
		if g.Phone == "" {
			add(StepGuardian, "phone", "Enter your parent or guardian's phone number.")
		}
		if !g.ConsentGiven {
			add(StepGuardian, "consent_given", "Your parent or guardian must agree before you apply, because you are under 18.")
		}
	}

	if len(d.Education) == 0 {
		add(StepEducation, "education", "Add your school or college, with your KCSE or equivalent results.")
	}
	for i, e := range d.Education {
		field := "education." + strconv.Itoa(i)
		if e.Institution == "" {
			add(StepEducation, field+".institution", "Enter the name of the school or college.")
		}
		if e.Level == "kcse" && e.MeanGrade == "" {
			add(StepEducation, field+".mean_grade", "Enter your KCSE mean grade.")
		}
	}

	switch {
	case d.Sponsor.Type == "":
		add(StepSponsor, "type", "Choose who will pay your fees.")
	case d.Sponsor.Type != "self" && d.Sponsor.Name == "":
		add(StepSponsor, "name", "Enter the name of whoever will pay your fees.")
	case d.Sponsor.Type != "self" && d.Sponsor.Phone == "":
		add(StepSponsor, "phone", "Enter their phone number.")
	}

	d.missingDocuments(add)
	return out
}

func trim(s string) string { return strings.Join(strings.Fields(s), " ") }

func maxLen(p *content.Problems, field, v string, n int) {
	if utf8.RuneCountInString(v) > n {
		p.Add(field, "Keep this under "+strconv.Itoa(n)+" characters.")
	}
}

func oneOfOrEmpty(p *content.Problems, field, v string, allowed []string, msg string) {
	if v != "" && !slices.Contains(allowed, v) {
		p.Add(field, msg)
	}
}

// SavePersonal saves the "About you" step. Empty fields are allowed while
// the draft is in progress; what is filled in must be well formed.
func (s *Service) SavePersonal(ctx context.Context, userID, id uuid.UUID, in Personal) error {
	in = Personal{
		FirstName: trim(in.FirstName), MiddleName: trim(in.MiddleName), LastName: trim(in.LastName),
		DateOfBirth: strings.TrimSpace(in.DateOfBirth), IdentityDocumentType: in.IdentityDocumentType,
		IdentityDocument: strings.ToUpper(trim(in.IdentityDocument)), Nationality: trim(in.Nationality),
		County: in.County, Phone: trim(in.Phone),
	}
	var p content.Problems
	maxLen(&p, "first_name", in.FirstName, 80)
	maxLen(&p, "middle_name", in.MiddleName, 80)
	maxLen(&p, "last_name", in.LastName, 80)
	var dob *time.Time
	if in.DateOfBirth != "" {
		t, err := time.Parse(time.DateOnly, in.DateOfBirth)
		switch {
		case err != nil:
			p.Add("date_of_birth", "Enter the date like 2008-03-21 (year, month, day).")
		case t.After(s.today()):
			p.Add("date_of_birth", "Your date of birth cannot be in the future.")
		case ageOn(t, s.today()) > 100:
			p.Add("date_of_birth", "Check the year: that would make you over 100.")
		default:
			dob = &t
		}
	}
	oneOfOrEmpty(&p, "identity_document_type", in.IdentityDocumentType, documentTypes, "Choose national ID, passport or birth certificate.")
	maxLen(&p, "identity_document", in.IdentityDocument, 30)
	if strings.ContainsFunc(in.IdentityDocument, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == ' ' || r == '-' || r == '/')
	}) {
		p.Add("identity_document", "Use only the letters, numbers, hyphens and slashes on the document.")
	}
	maxLen(&p, "nationality", in.Nationality, 60)
	oneOfOrEmpty(&p, "county", in.County, Counties, "Choose a county from the list.")
	p.Phone("phone", in.Phone)
	if err := p.Err(); err != nil {
		return err
	}
	return s.edit(ctx, userID, id, func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error {
		var docType, phone *string
		if in.IdentityDocumentType != "" {
			docType = &in.IdentityDocumentType
		}
		if in.Phone != "" {
			phone = &in.Phone
		}
		return q.UpdateApplicantPersonal(ctx, dbq.UpdateApplicantPersonalParams{
			ID: a.ApplicantID, FirstName: in.FirstName, MiddleName: in.MiddleName, LastName: in.LastName,
			DateOfBirth: dob, IdentityDocumentType: docType, IdentityDocument: in.IdentityDocument,
			Nationality: in.Nationality, County: in.County, Phone: phone,
		})
	})
}

// SaveGuardian saves the parent or guardian step and records consent
// changes. Consent is given in the guardian's name; changing the name
// means asking again.
func (s *Service) SaveGuardian(ctx context.Context, userID, id uuid.UUID, in Guardian, req audit.Request) error {
	in = Guardian{FullName: trim(in.FullName), Relationship: in.Relationship, Phone: trim(in.Phone), Email: strings.TrimSpace(in.Email), ConsentGiven: in.ConsentGiven}
	var p content.Problems
	maxLen(&p, "full_name", in.FullName, 120)
	oneOfOrEmpty(&p, "relationship", in.Relationship, relationships, "Choose how they are related to you.")
	p.Phone("phone", in.Phone)
	p.Email("email", in.Email)
	maxLen(&p, "email", in.Email, 254)
	if in.ConsentGiven && in.FullName == "" {
		p.Add("consent_given", "Enter your parent or guardian's name first.")
	}
	if err := p.Err(); err != nil {
		return err
	}
	return s.edit(ctx, userID, id, func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error {
		var email *string
		if in.Email != "" {
			email = &in.Email
		}
		guardianID, err := q.UpsertGuardian(ctx, dbq.UpsertGuardianParams{
			ApplicationID: a.ID, FullName: in.FullName, Relationship: in.Relationship, Phone: in.Phone, Email: email,
		})
		if err != nil {
			return err
		}
		holds, err := consentHolds(ctx, q, guardianID, in.FullName)
		if err != nil || holds == in.ConsentGiven {
			return err
		}
		// Reaching here means it changed: given now, or withdrawn.
		var ua *string
		if req.UserAgent != "" {
			ua = &req.UserAgent
		}
		if err := q.InsertConsent(ctx, dbq.InsertConsentParams{
			SubjectType: "guardian", SubjectID: guardianID, Purpose: "guardian_consent", Given: in.ConsentGiven,
			NoticeVersion: PrivacyNoticeVersion, GivenByName: in.FullName, Ip: req.IP, UserAgent: ua,
		}); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "application.guardian_consent", EntityType: "application", EntityID: a.ID.String(),
			Details: map[string]any{"given": in.ConsentGiven, "notice_version": PrivacyNoticeVersion}})
	})
}

// SaveEducation replaces the education history.
func (s *Service) SaveEducation(ctx context.Context, userID, id uuid.UUID, items []Education) error {
	var p content.Problems
	if len(items) > MaxEducationEntries {
		p.Add("education", "Add at most "+strconv.Itoa(MaxEducationEntries)+" schools or colleges.")
	}
	thisYear := s.today().Year()
	for i := range items {
		e := &items[i]
		f := "education." + strconv.Itoa(i)
		e.Institution, e.Qualification, e.MeanGrade = trim(e.Institution), trim(e.Qualification), strings.ToUpper(trim(e.MeanGrade))
		if !slices.Contains(educationLevel, e.Level) {
			p.Add(f+".level", "Choose the level: KCSE, KCPE, certificate, diploma, degree or other.")
		}
		maxLen(&p, f+".institution", e.Institution, 120)
		maxLen(&p, f+".qualification", e.Qualification, 120)
		if e.YearCompleted != nil && (*e.YearCompleted < 1950 || *e.YearCompleted > thisYear) {
			p.Add(f+".year_completed", "Enter the year you finished, from 1950 to "+strconv.Itoa(thisYear)+".")
		}
		if e.Level == "kcse" {
			oneOfOrEmpty(&p, f+".mean_grade", e.MeanGrade, KCSEGrades, "Enter a KCSE grade from A to E, like C+.")
		} else {
			maxLen(&p, f+".mean_grade", e.MeanGrade, 20)
		}
		if len(e.Subjects) > MaxSubjectGrades {
			p.Add(f+".subjects", "Add at most "+strconv.Itoa(MaxSubjectGrades)+" subjects.")
		}
		kept := []SubjectGrade{}
		for j, sg := range e.Subjects {
			sg = SubjectGrade{Subject: trim(sg.Subject), Grade: strings.ToUpper(trim(sg.Grade))}
			if sg.Subject == "" && sg.Grade == "" {
				continue
			}
			sf := f + ".subjects." + strconv.Itoa(j)
			maxLen(&p, sf+".subject", sg.Subject, 60)
			if e.Level == "kcse" {
				oneOfOrEmpty(&p, sf+".grade", sg.Grade, KCSEGrades, "Enter a KCSE grade from A to E, like B-.")
			} else {
				maxLen(&p, sf+".grade", sg.Grade, 10)
			}
			kept = append(kept, sg)
		}
		e.Subjects = kept
	}
	if err := p.Err(); err != nil {
		return err
	}
	return s.edit(ctx, userID, id, func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error {
		if err := q.DeleteEducation(ctx, a.ID); err != nil {
			return err
		}
		for i, e := range items {
			grades, err := json.Marshal(e.Subjects)
			if err != nil {
				return err
			}
			var year *int16
			if e.YearCompleted != nil {
				y := int16(*e.YearCompleted)
				year = &y
			}
			if err := q.InsertEducation(ctx, dbq.InsertEducationParams{
				ApplicationID: a.ID, Level: e.Level, Institution: e.Institution, YearCompleted: year,
				Qualification: e.Qualification, MeanGrade: e.MeanGrade, SubjectGrades: grades, SortOrder: int32(i),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveSponsor saves who will pay the fees.
func (s *Service) SaveSponsor(ctx context.Context, userID, id uuid.UUID, in Sponsor) error {
	in = Sponsor{Type: in.Type, Name: trim(in.Name), Phone: trim(in.Phone)}
	if in.Type == "self" {
		in.Name, in.Phone = "", ""
	}
	var p content.Problems
	oneOfOrEmpty(&p, "type", in.Type, sponsorTypes, "Choose who will pay your fees.")
	maxLen(&p, "name", in.Name, 120)
	p.Phone("phone", in.Phone)
	if err := p.Err(); err != nil {
		return err
	}
	return s.edit(ctx, userID, id, func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error {
		var t *string
		if in.Type != "" {
			t = &in.Type
		}
		return q.UpdateApplicationSponsor(ctx, dbq.UpdateApplicationSponsorParams{ID: a.ID, SponsorType: t, SponsorName: in.Name, SponsorPhone: in.Phone})
	})
}

// SaveIntake moves the application to another open intake of its programme.
func (s *Service) SaveIntake(ctx context.Context, userID, id, intakeID uuid.UUID) error {
	err := s.edit(ctx, userID, id, func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error {
		open, err := q.ListOpenIntakes(ctx, dbq.ListOpenIntakesParams{ProgrammeID: a.ProgrammeID, Today: s.today()})
		if err != nil {
			return err
		}
		if !containsIntake(open, intakeID) {
			return content.Problems{{Field: "intake_id", Message: "That intake is not taking applications. Choose another."}}.Err()
		}
		return q.UpdateApplicationIntake(ctx, dbq.UpdateApplicationIntakeParams{ID: a.ID, IntakeID: &intakeID})
	})
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return err
}
