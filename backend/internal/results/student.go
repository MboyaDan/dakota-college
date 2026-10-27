package results

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/academics"
	"github.com/MboyaDan/dakota-college/backend/internal/application"
	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/letters"
)

// What a student sees (AC-09): only released results, as frozen at the
// newest release of each class.

type StudentResult struct {
	TermName  string    `json:"term_name"`
	UnitCode  string    `json:"unit_code"`
	UnitTitle string    `json:"unit_title"`
	Released  time.Time `json:"released_at"`
	Version   int32     `json:"version"`
	Total     *int      `json:"total"`
	Grade     string    `json:"grade"`
	Passed    bool      `json:"passed"`
	Outcome   *string   `json:"outcome"`
	startsOn  time.Time
	unitHours *int32
}

func released(ctx context.Context, q *dbq.Queries, student uuid.UUID) ([]StudentResult, error) {
	rows, err := q.StudentReleasedResults(ctx, student.String())
	if err != nil {
		return nil, err
	}
	out := make([]StudentResult, 0, len(rows))
	for _, r := range rows {
		var rel Released
		if err := json.Unmarshal([]byte(r.Result), &rel); err != nil {
			return nil, err
		}
		out = append(out, StudentResult{TermName: r.TermName, UnitCode: r.UnitCode, UnitTitle: r.UnitTitle, Released: r.ReleasedAt,
			Version: r.Version, Total: rel.Total, Grade: rel.Grade, Passed: rel.Passed, Outcome: rel.Outcome})
	}
	return out, nil
}

func (s *Service) ownStudent(ctx context.Context, q *dbq.Queries, user uuid.UUID) (dbq.GetStudentByUserRow, error) {
	st, err := q.GetStudentByUser(ctx, user)
	if errors.Is(err, pgx.ErrNoRows) {
		return st, ErrNoStudent
	}
	return st, err
}

// StudentResults are the signed-in student's released results.
func (s *Service) StudentResults(ctx context.Context, user uuid.UUID) ([]StudentResult, error) {
	q := dbq.New(s.pool)
	st, err := s.ownStudent(ctx, q, user)
	if err != nil {
		return nil, err
	}
	return released(ctx, q, st.ID)
}

func outcomeLabel(r StudentResult) (mark, grade string) {
	if r.Outcome != nil {
		if *r.Outcome == "competent" {
			return "", "Competent"
		}
		return "", "Not yet competent"
	}
	if r.Total != nil {
		mark = fmt.Sprint(*r.Total)
	}
	return mark, r.Grade
}

func document(st dbq.GetStudentByUserRow, rs []StudentResult, now time.Time, official bool, number string, sample bool) ([]byte, error) {
	doc := letters.Results{Official: official, Number: number, Date: now.In(nairobi), StudentNumber: st.StudentNumber,
		Name: fullName(st.FirstName, st.MiddleName, st.LastName), Programme: st.ProgrammeTitle, Sample: sample}
	for _, r := range rs {
		if len(doc.Terms) == 0 || doc.Terms[len(doc.Terms)-1].Name != r.TermName {
			doc.Terms = append(doc.Terms, letters.ResultTerm{Name: r.TermName})
		}
		mark, grade := outcomeLabel(r)
		t := &doc.Terms[len(doc.Terms)-1]
		t.Rows = append(t.Rows, letters.ResultRow{UnitCode: r.UnitCode, UnitTitle: r.UnitTitle, Mark: mark, Grade: grade})
	}
	return letters.ResultsDocument(doc)
}

// ResultsSlip is a provisional PDF of the student's released results.
func (s *Service) ResultsSlip(ctx context.Context, user uuid.UUID) ([]byte, string, error) {
	q := dbq.New(s.pool)
	st, err := s.ownStudent(ctx, q, user)
	if err != nil {
		return nil, "", err
	}
	rs, err := released(ctx, q, st.ID)
	if err != nil {
		return nil, "", err
	}
	pdf, err := document(st, rs, s.now(), false, "", isSample(st.StudentNumber))
	return pdf, "Results slip " + st.StudentNumber + ".pdf", err
}

// ---------- official transcripts ----------

type Transcript struct {
	ID           uuid.UUID `json:"id"`
	Number       string    `json:"number"`
	IssuedAt     time.Time `json:"issued_at"`
	IssuedByName string    `json:"issued_by_name"`
}

func listTranscripts(ctx context.Context, q *dbq.Queries, student uuid.UUID) ([]Transcript, error) {
	rows, err := q.ListTranscripts(ctx, student)
	if err != nil {
		return nil, err
	}
	out := make([]Transcript, 0, len(rows))
	for _, r := range rows {
		out = append(out, Transcript{ID: r.ID, Number: r.Number, IssuedAt: r.IssuedAt, IssuedByName: r.IssuedByName})
	}
	return out, nil
}

// StudentTranscripts are the official transcripts issued to the student.
func (s *Service) StudentTranscripts(ctx context.Context, user uuid.UUID) ([]Transcript, error) {
	q := dbq.New(s.pool)
	st, err := s.ownStudent(ctx, q, user)
	if err != nil {
		return nil, err
	}
	return listTranscripts(ctx, q, st.ID)
}

// StudentTranscriptLink gives the student a short-lived link to one of their transcripts.
func (s *Service) StudentTranscriptLink(ctx context.Context, user, id uuid.UUID) (*application.FileLink, error) {
	q := dbq.New(s.pool)
	st, err := s.ownStudent(ctx, q, user)
	if err != nil {
		return nil, err
	}
	file, err := q.GetTranscriptFile(ctx, dbq.GetTranscriptFileParams{ID: id, StudentID: st.ID})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return application.IssueLink(ctx, q, file, user, s.now())
}

// studentInScope finds a student record in scope, as their own row type.
func studentInScope(ctx context.Context, q *dbq.Queries, sc academics.Scope, id uuid.UUID) (dbq.GetStudentByUserRow, error) {
	r, err := q.StaffGetStudent(ctx, dbq.StaffGetStudentParams{ID: id, AllSchools: sc.All, Schools: sc.Schools})
	if errors.Is(err, pgx.ErrNoRows) {
		return dbq.GetStudentByUserRow{}, ErrNotFound
	}
	if err != nil {
		return dbq.GetStudentByUserRow{}, err
	}
	return dbq.GetStudentByUserRow{ID: r.ID, StudentNumber: r.StudentNumber, FirstName: r.FirstName, MiddleName: r.MiddleName,
		LastName: r.LastName, ProgrammeTitle: r.ProgrammeTitle}, nil
}

// StaffTranscripts lists a student's official transcripts.
func (s *Service) StaffTranscripts(ctx context.Context, sc academics.Scope, student uuid.UUID) ([]Transcript, error) {
	q := dbq.New(s.pool)
	if _, err := studentInScope(ctx, q, sc, student); err != nil {
		return nil, err
	}
	return listTranscripts(ctx, q, student)
}

// IssueTranscript makes an official transcript of the student's released
// results: numbered DTC/TR/YYYY/NNNN (gapless), kept privately. If it
// answers a transcript request, that request is marked done.
func (s *Service) IssueTranscript(ctx context.Context, p *auth.Principal, sc academics.Scope, student uuid.UUID, request *uuid.UUID, req audit.Request) (*Transcript, error) {
	if s.storage == nil {
		return nil, ErrStorageOff
	}
	var out Transcript
	var key string
	stored := false
	err := s.tx(ctx, func(q *dbq.Queries) error {
		st, err := studentInScope(ctx, q, sc, student)
		if err != nil {
			return err
		}
		rs, err := released(ctx, q, student)
		if err != nil {
			return err
		}
		if len(rs) == 0 {
			return content.Problems{{Field: "student", Message: "This student has no released results yet."}}.Err()
		}
		year := s.now().In(nairobi).Year()
		n, err := q.NextCounterValue(ctx, dbq.NextCounterValueParams{Name: "transcript", Period: int32(year)})
		if err != nil {
			return err
		}
		number := fmt.Sprintf("DTC/TR/%d/%04d", year, n)
		pdf, err := document(st, rs, s.now(), true, number, isSample(st.StudentNumber))
		if err != nil {
			return err
		}
		key = fmt.Sprintf("transcripts/%s/%s.pdf", student, uuid.NewString())
		if err := s.storage.PutPrivate(ctx, s.bucket, key, pdf, "application/pdf"); err != nil {
			return fmt.Errorf("store transcript: %w", err)
		}
		stored = true
		sum := sha256.Sum256(pdf)
		file, err := q.InsertPrivateFile(ctx, dbq.InsertPrivateFileParams{Bucket: s.bucket, ObjectKey: key,
			OriginalName: "Transcript " + number + ".pdf", ContentType: "application/pdf", SizeBytes: int64(len(pdf)), Sha256: sum[:],
			ScanStatus: "not_scanned", UploadedByUserID: &p.UserID})
		if err != nil {
			return err
		}
		if request != nil {
			r, err := q.LockServiceRequest(ctx, *request)
			if errors.Is(err, pgx.ErrNoRows) || (err == nil && (r.StudentID != student || r.Kind != "transcript" || r.Status == "done" || r.Status == "declined")) {
				return content.Problems{{Field: "service_request_id", Message: "Choose an open transcript request of this student."}}.Err()
			}
			if err != nil {
				return err
			}
			if err := q.UpdateServiceRequest(ctx, dbq.UpdateServiceRequestParams{ID: *request, Status: "done",
				Response: "Official transcript " + number + " issued. Download it from Results.", HandledBy: &p.UserID}); err != nil {
				return err
			}
		}
		row, err := q.InsertTranscript(ctx, dbq.InsertTranscriptParams{Number: number, StudentID: student, FileID: file, ServiceRequestID: request, IssuedBy: p.UserID})
		if err != nil {
			return err
		}
		out = Transcript{ID: row.ID, Number: number, IssuedAt: row.IssuedAt, IssuedByName: p.FullName}
		return audit.Write(ctx, q, req, audit.Entry{Action: "results.transcript_issued", EntityType: "student", EntityID: student.String(),
			Details: map[string]any{"number": number, "units": len(rs)}})
	})
	if err != nil {
		if stored {
			if derr := s.storage.Delete(context.WithoutCancel(ctx), s.bucket, key); derr != nil {
				s.logger.Error("could not remove unused transcript", slog.String("key", key), slog.String("error", derr.Error()))
			}
		}
		return nil, err
	}
	return &out, nil
}

// StaffTranscriptLink gives staff a short-lived link to a transcript. Audited.
func (s *Service) StaffTranscriptLink(ctx context.Context, p *auth.Principal, sc academics.Scope, student, id uuid.UUID, req audit.Request) (*application.FileLink, error) {
	var link *application.FileLink
	err := s.tx(ctx, func(q *dbq.Queries) error {
		if _, err := studentInScope(ctx, q, sc, student); err != nil {
			return err
		}
		file, err := q.GetTranscriptFile(ctx, dbq.GetTranscriptFileParams{ID: id, StudentID: student})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if link, err = application.IssueLink(ctx, q, file, p.UserID, s.now()); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "file.link_issued", EntityType: "student", EntityID: student.String(),
			Details: map[string]any{"file": "transcript", "transcript_id": id.String()}})
	})
	return link, err
}

// isSample: test students from the dev seed, whose documents say so.
func isSample(number string) bool { return strings.HasPrefix(number, "DTC/SAMPLE/") }

// StaffResults are a student's released results, for staff in scope.
func (s *Service) StaffResults(ctx context.Context, sc academics.Scope, student uuid.UUID) ([]StudentResult, error) {
	q := dbq.New(s.pool)
	if _, err := studentInScope(ctx, q, sc, student); err != nil {
		return nil, err
	}
	return released(ctx, q, student)
}
