package application

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
)

// Submission (brief 8, AC-04, ADR-07). The browser sends an Idempotency-Key
// with each submit attempt; a retry with the same key gets the first answer
// back, and an application is submitted at most once whatever the keys.

const idempotencyTTL = 24 * time.Hour

var (
	// ErrKeyReused means the key was used for a different request.
	ErrKeyReused = errors.New("idempotency key reused for another request")
	// ErrAlreadySubmitted: submitted before, under another key.
	ErrAlreadySubmitted = errors.New("application already submitted")
)

// SubmitResult is the stored answer to a submission.
type SubmitResult struct {
	Status int
	Body   json.RawMessage
	// Replayed is true when this is the stored answer to an earlier request.
	Replayed bool
}

// Submit checks the declaration and that nothing is missing, then submits
// the draft in one transaction: status, frozen copy, history, consent,
// audit and the idempotency record all commit together or not at all.
func (s *Service) Submit(ctx context.Context, p *auth.Principal, id uuid.UUID, key string, declaration bool, req audit.Request) (*SubmitResult, error) {
	if len(key) < 8 || len(key) > 128 {
		return nil, content.Problems{{Field: "idempotency_key", Message: "Reload the page and submit again."}}.Err()
	}
	scope := "application.submit:" + p.UserID.String()
	reqHash := sha256.Sum256([]byte(id.String() + "|" + boolString(declaration)))

	var out *SubmitResult
	var reference, programme string
	err := s.tx(ctx, func(q *dbq.Queries) error {
		n, err := q.ClaimIdempotencyKey(ctx, dbq.ClaimIdempotencyKeyParams{
			Scope: scope, Key: key, RequestHash: reqHash[:], ExpiresAt: s.now().Add(idempotencyTTL),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			rec, err := q.GetIdempotencyRecord(ctx, dbq.GetIdempotencyRecordParams{Scope: scope, Key: key})
			if err != nil {
				return err
			}
			if string(rec.RequestHash) != string(reqHash[:]) {
				return ErrKeyReused
			}
			if rec.CompletedAt == nil || rec.ResponseStatus == nil {
				// Cannot happen: records are completed in the same transaction.
				return errors.New("idempotency record without a response")
			}
			// jsonb reformats what it stores; encode it again the way the first
			// answer was encoded, so a replay is byte for byte the same.
			var stored map[string]any
			if err := json.Unmarshal(rec.ResponseBody, &stored); err != nil {
				return err
			}
			body, err := json.Marshal(stored)
			if err != nil {
				return err
			}
			out = &SubmitResult{Status: int(*rec.ResponseStatus), Body: body, Replayed: true}
			return nil
		}

		lk, err := q.LockOwnApplication(ctx, dbq.LockOwnApplicationParams{ID: id, UserID: &p.UserID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if s.due(lk.Status, lk.DraftExpiresAt) || lk.Status == "expired" {
			return ErrNotEditable
		}
		if lk.Status != "draft" {
			return ErrAlreadySubmitted
		}

		d, err := s.Get(ctx, p.UserID, id)
		if err != nil {
			return err
		}
		var problems content.Problems
		if !declaration {
			problems.Add("declaration", "Tick the declaration to confirm your answers are true.")
		}
		for _, m := range d.Missing {
			problems.Add(m.Step+"."+m.Field, m.Message)
		}
		if err := problems.Err(); err != nil {
			return err
		}

		snapshot, err := json.Marshal(newSnapshot(d, s.now()))
		if err != nil {
			return err
		}
		if n, err := q.SubmitApplication(ctx, dbq.SubmitApplicationParams{ID: id, Snapshot: snapshot}); err != nil || n != 1 {
			if err == nil {
				err = errors.New("application changed while submitting")
			}
			return err
		}
		from := "draft"
		if err := q.InsertStatusHistory(ctx, dbq.InsertStatusHistoryParams{ApplicationID: id, FromStatus: &from, ToStatus: "submitted", Reason: "Submitted online"}); err != nil {
			return err
		}
		fullName := d.Personal.FirstName + " " + d.Personal.LastName
		var ua *string
		if req.UserAgent != "" {
			ua = &req.UserAgent
		}
		if err := q.InsertConsent(ctx, dbq.InsertConsentParams{
			SubjectType: "applicant", SubjectID: lk.ApplicantID, Purpose: "data_processing", Given: true,
			NoticeVersion: PrivacyNoticeVersion, GivenByName: fullName, Ip: req.IP, UserAgent: ua,
		}); err != nil {
			return err
		}
		if err := audit.Write(ctx, q, req, audit.Entry{Action: "application.submitted", EntityType: "application", EntityID: id.String(),
			Details: map[string]any{"reference": d.Reference, "documents": len(d.Documents)}}); err != nil {
			return err
		}

		body, err := json.Marshal(map[string]any{"id": id, "reference": d.Reference, "status": "submitted"})
		if err != nil {
			return err
		}
		status := int32(http.StatusOK)
		if err := q.CompleteIdempotencyKey(ctx, dbq.CompleteIdempotencyKeyParams{Scope: scope, Key: key, ResponseStatus: &status, ResponseBody: body}); err != nil {
			return err
		}
		out = &SubmitResult{Status: http.StatusOK, Body: body}
		reference, programme = d.Reference, d.ProgrammeTitle
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !out.Replayed {
		s.sendSubmitted(ctx, p, id, reference, programme)
	}
	return out, nil
}

// sendSubmitted confirms receipt by email (and SMS once DG-03 clears).
func (s *Service) sendSubmitted(ctx context.Context, p *auth.Principal, id uuid.UUID, reference, programme string) {
	ap, err := dbq.New(s.pool).GetApplicantByUser(ctx, &p.UserID)
	if err != nil {
		s.logger.Error("submitted notice: applicant not found", slog.String("error", err.Error()))
		return
	}
	vars := map[string]string{
		"name": ap.FirstName, "reference": reference, "programme": programme,
		"link": s.appBaseURL + "/apply/application?id=" + id.String(),
	}
	m := notify.Message{Template: notify.ApplicationSubmitted, To: p.Email, Vars: vars, RelatedType: "application", RelatedID: id.String()}
	if err := s.notify.Email(ctx, m); err != nil {
		s.logger.Error("submitted email not queued", slog.String("error", err.Error()))
	}
	if ap.Phone != nil {
		m.To = *ap.Phone
		if err := s.notify.SMS(ctx, m); err != nil {
			s.logger.Error("submitted SMS not recorded", slog.String("error", err.Error()))
		}
	}
}

func boolString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// Snapshot is the application as submitted: what the applicant declared
// true, kept unchanged for the record (staff review it in Stage 10).
type Snapshot struct {
	Reference      string      `json:"reference"`
	SubmittedAt    time.Time   `json:"submitted_at"`
	ProgrammeTitle string      `json:"programme_title"`
	ProgrammeSlug  string      `json:"programme_slug"`
	IntakeID       *uuid.UUID  `json:"intake_id"`
	IntakeName     *string     `json:"intake_name"`
	IntakeStartsOn *string     `json:"intake_starts_on"`
	Email          string      `json:"email"`
	Personal       Personal    `json:"personal"`
	Guardian       *Guardian   `json:"guardian"`
	Education      []Education `json:"education"`
	Sponsor        Sponsor     `json:"sponsor"`
	Documents      []Document  `json:"documents"`
}

func newSnapshot(d *Detail, at time.Time) Snapshot {
	snap := Snapshot{
		Reference: d.Reference, SubmittedAt: at, ProgrammeTitle: d.ProgrammeTitle, ProgrammeSlug: d.ProgrammeSlug,
		IntakeID: d.IntakeID, IntakeName: d.IntakeName, IntakeStartsOn: d.IntakeStartsOn, Email: d.Email,
		Personal: d.Personal, Education: d.Education, Sponsor: d.Sponsor, Documents: d.Documents,
	}
	if d.NeedsGuardian {
		g := d.Guardian
		snap.Guardian = &g
	}
	return snap
}
