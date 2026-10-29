package privacy

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

type ApplicantMatch struct {
	ID           uuid.UUID  `json:"id"`
	Name         string     `json:"name"`
	Email        string     `json:"email"`
	Phone        *string    `json:"phone"`
	CreatedAt    time.Time  `json:"created_at"`
	Applications int64      `json:"applications"`
	IsStudent    bool       `json:"is_student"`
	InProgress   int64      `json:"in_progress"`
	ErasedAt     *time.Time `json:"erased_at"`
	// Erasable says whether Erase would accept this applicant now.
	Erasable bool `json:"erasable"`
	userID   *uuid.UUID
}

type StudentMatch struct {
	ID            uuid.UUID `json:"id"`
	StudentNumber string    `json:"student_number"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Phone         *string   `json:"phone"`
	Status        string    `json:"status"`
	EnrolledAt    time.Time `json:"enrolled_at"`
	userID        uuid.UUID
}

type EnquiryMatch struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     *string   `json:"email"`
	Phone     *string   `json:"phone"`
	QueryType string    `json:"query_type"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// GuardianMatch is the person named as an applicant's or student's parent
// or guardian. Those details stay with the applicant's or student's record.
type GuardianMatch struct {
	Source string  `json:"source"`
	Record string  `json:"record"`
	Name   string  `json:"name"`
	Email  *string `json:"email"`
	Phone  string  `json:"phone"`
}

type StaffMatch struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Matches are the records that carry a request's email or phone. Staff
// confirm they belong to the person before acting on them.
type Matches struct {
	Applicants []ApplicantMatch `json:"applicants"`
	Students   []StudentMatch   `json:"students"`
	Enquiries  []EnquiryMatch   `json:"enquiries"`
	Guardians  []GuardianMatch  `json:"guardians"`
	Staff      []StaffMatch     `json:"staff"`
}

// phoneTail is the last nine digits of a phone number, which is how a
// Kenyan number reads whether written 07..., 2547... or +254 7...
func phoneTail(phone *string) *string {
	if phone == nil {
		return nil
	}
	var b strings.Builder
	for _, r := range *phone {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	d := b.String()
	if len(d) < 9 {
		return nil
	}
	t := d[len(d)-9:]
	return &t
}

func fullName(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

func (s *Service) find(ctx context.Context, q *dbq.Queries, email, phone *string) (*Matches, error) {
	tail := phoneTail(phone)
	m := &Matches{}
	apps, err := q.FindApplicantsByContact(ctx, dbq.FindApplicantsByContactParams{Email: email, PhoneTail: tail})
	if err != nil {
		return nil, err
	}
	for _, a := range apps {
		m.Applicants = append(m.Applicants, ApplicantMatch{
			ID: a.ID, Name: fullName(a.FirstName, a.MiddleName, a.LastName), Email: a.Email, Phone: a.Phone, CreatedAt: a.CreatedAt,
			Applications: a.Applications, IsStudent: a.IsStudent, InProgress: a.InProgress, ErasedAt: a.ErasedAt,
			Erasable: a.ErasedAt == nil && !a.IsStudent && a.InProgress == 0, userID: a.UserID,
		})
	}
	students, err := q.FindStudentsByContact(ctx, dbq.FindStudentsByContactParams{Email: email, PhoneTail: tail})
	if err != nil {
		return nil, err
	}
	for _, st := range students {
		m.Students = append(m.Students, StudentMatch{
			ID: st.ID, StudentNumber: st.StudentNumber, Name: fullName(st.FirstName, st.MiddleName, st.LastName),
			Email: st.Email, Phone: st.Phone, Status: st.Status, EnrolledAt: st.EnrolledAt, userID: st.UserID,
		})
	}
	enq, err := q.FindEnquiriesByContact(ctx, dbq.FindEnquiriesByContactParams{Email: email, PhoneTail: tail})
	if err != nil {
		return nil, err
	}
	for _, e := range enq {
		m.Enquiries = append(m.Enquiries, EnquiryMatch{ID: e.ID, Name: e.FullName, Email: e.Email, Phone: e.Phone, QueryType: e.QueryType, Status: e.Status, CreatedAt: e.CreatedAt})
	}
	gs, err := q.FindGuardianRecordsByContact(ctx, dbq.FindGuardianRecordsByContactParams{Email: email, PhoneTail: tail})
	if err != nil {
		return nil, err
	}
	for _, g := range gs {
		m.Guardians = append(m.Guardians, GuardianMatch{Source: g.Source, Record: g.Record, Name: g.FullName, Email: g.Email, Phone: g.Phone})
	}
	staff, err := q.FindStaffByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	for _, u := range staff {
		m.Staff = append(m.Staff, StaffMatch{ID: u.ID, Name: u.FullName, Email: u.Email, Status: u.Status, CreatedAt: u.CreatedAt})
	}
	return m, nil
}

// ExportFile is everything held about the person, for an access or
// portability request. Staff read it before sending it on, and remove
// anything about other people (privacy procedure, docs/privacy-requests.md).
type ExportFile struct {
	Name string
	Data []byte
}

type exportDoc struct {
	Reference   string            `json:"request_reference"`
	PreparedAt  time.Time         `json:"prepared_at"`
	About       string            `json:"about"`
	SearchedFor map[string]any    `json:"searched_for"`
	Applicants  []json.RawMessage `json:"applications"`
	Students    []json.RawMessage `json:"student_records"`
	Enquiries   []json.RawMessage `json:"enquiries"`
	Guardian    []GuardianMatch   `json:"named_as_parent_or_guardian"`
	Staff       []json.RawMessage `json:"staff_accounts"`
	Messages    json.RawMessage   `json:"messages_sent"`
}

const exportAbout = "Personal data Dakota Training College holds about you, found by the email address and phone number in your request. " +
	"Dates and times are in UTC. Amounts are in Kenya shilling cents (100 cents = KES 1)."

// Export builds the JSON file for an access or portability request. The
// request must be open and identity checked. It is logged on the request
// and audited.
func (s *Service) Export(ctx context.Context, actor *auth.Principal, req audit.Request, id uuid.UUID) (*ExportFile, error) {
	var file *ExportFile
	err := s.act(ctx, actor, req, id, "exported", "", func(q *dbq.Queries, d dbq.LockDataRequestRow) (map[string]any, error) {
		if d.Kind != "access" && d.Kind != "portability" {
			return nil, ErrWrongKind
		}
		if d.IdentityCheckedAt == nil {
			return nil, ErrIdentityNotChecked
		}
		m, err := s.find(ctx, q, d.SubjectEmail, d.SubjectPhone)
		if err != nil {
			return nil, err
		}
		doc := exportDoc{
			Reference: d.Reference, PreparedAt: s.now().UTC(), About: exportAbout,
			SearchedFor: map[string]any{"email": d.SubjectEmail, "phone": d.SubjectPhone},
			Applicants:  []json.RawMessage{}, Students: []json.RawMessage{}, Enquiries: []json.RawMessage{},
			Guardian: m.Guardians, Staff: []json.RawMessage{},
		}
		if doc.Guardian == nil {
			doc.Guardian = []GuardianMatch{}
		}
		withAccount := func(record []byte, userID *uuid.UUID) (json.RawMessage, error) {
			if userID == nil {
				return record, nil
			}
			acct, err := q.ExportUserAccount(ctx, *userID)
			if err != nil {
				return nil, err
			}
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(record, &obj); err != nil {
				return nil, err
			}
			obj["account"] = acct
			return json.Marshal(obj)
		}
		for _, a := range m.Applicants {
			rec, err := q.ExportApplicant(ctx, a.ID)
			if err != nil {
				return nil, err
			}
			if rec, err = withAccount(rec, a.userID); err != nil {
				return nil, err
			}
			doc.Applicants = append(doc.Applicants, rec)
		}
		for _, st := range m.Students {
			rec, err := q.ExportStudent(ctx, st.ID)
			if err != nil {
				return nil, err
			}
			uid := st.userID
			if rec, err = withAccount(rec, &uid); err != nil {
				return nil, err
			}
			doc.Students = append(doc.Students, rec)
		}
		for _, e := range m.Enquiries {
			rec, err := q.ExportEnquiry(ctx, e.ID)
			if err != nil {
				return nil, err
			}
			doc.Enquiries = append(doc.Enquiries, rec)
		}
		for _, u := range m.Staff {
			rec, err := q.ExportUserAccount(ctx, u.ID)
			if err != nil {
				return nil, err
			}
			doc.Staff = append(doc.Staff, rec)
		}
		if doc.Messages, err = q.ExportMessagesTo(ctx, dbq.ExportMessagesToParams{Email: d.SubjectEmail, PhoneTail: phoneTail(d.SubjectPhone)}); err != nil {
			return nil, err
		}
		data, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return nil, err
		}
		file = &ExportFile{Name: "dakota-" + strings.ToLower(d.Reference) + "-data.json", Data: append(data, '\n')}
		return map[string]any{
			"applicants": len(m.Applicants), "students": len(m.Students), "enquiries": len(m.Enquiries),
			"guardian_records": len(m.Guardians), "staff_accounts": len(m.Staff),
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return file, nil
}
