package admissions

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/application"
	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/letters"
	"github.com/MboyaDan/dakota-college/backend/internal/notify"
)

// Decisions (brief 8). Reasons written here are shown to the applicant in
// their status history (D-115); internal remarks belong in notes.

// RequestInfo asks the applicant for more; the application waits for them.
func (s *Service) RequestInfo(ctx context.Context, p *auth.Principal, id uuid.UUID, message string, req audit.Request) error {
	message = strings.TrimSpace(message)
	var pr content.Problems
	pr.Required("message", message, maxTextLength)
	if err := pr.Err(); err != nil {
		return err
	}
	var to recipient
	err := s.change(ctx, id, func(q *dbq.Queries, a dbq.LockApplicationRow) error {
		if a.Status != "under_review" {
			return application.ErrIllegalTransition
		}
		r, err := q.InsertInfoRequest(ctx, dbq.InsertInfoRequestParams{ApplicationID: id, Message: message, RequestedBy: p.UserID})
		if err != nil {
			return err
		}
		if err := application.Move(ctx, q, id, a.Status, "info_requested", &p.UserID, "More information requested"); err != nil {
			return err
		}
		if to, err = s.recipient(ctx, q, a); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("application.info_requested", id, map[string]any{"request_id": r.ID.String()}))
	})
	if err != nil {
		return err
	}
	s.tell(ctx, notify.ApplicationInfoRequested, id, to, map[string]string{"message": message})
	return nil
}

// ResumeReview takes the application back into review without waiting for
// the applicant (for example, they answered by phone). The open request is
// closed in the reviewer's name.
func (s *Service) ResumeReview(ctx context.Context, p *auth.Principal, id uuid.UUID, reason string, req audit.Request) error {
	reason = strings.TrimSpace(reason)
	var pr content.Problems
	pr.Optional("reason", reason, maxTextLength)
	if err := pr.Err(); err != nil {
		return err
	}
	if reason == "" {
		reason = "Review resumed by the admissions office"
	}
	return s.change(ctx, id, func(q *dbq.Queries, a dbq.LockApplicationRow) error {
		if a.Status != "info_requested" {
			return application.ErrIllegalTransition
		}
		if err := closeOpenRequest(ctx, q, id, &p.UserID); err != nil {
			return err
		}
		if err := application.Move(ctx, q, id, a.Status, "under_review", &p.UserID, reason); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("application.review_resumed", id, nil))
	})
}

func closeOpenRequest(ctx context.Context, q *dbq.Queries, id uuid.UUID, by *uuid.UUID) error {
	r, err := q.GetOpenInfoRequest(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = q.CloseInfoRequest(ctx, dbq.CloseInfoRequestParams{ID: r.ID, ClosedBy: by})
	return err
}

// Reject ends the application with a reason the applicant will read.
func (s *Service) Reject(ctx context.Context, p *auth.Principal, id uuid.UUID, reason string, req audit.Request) error {
	reason = strings.TrimSpace(reason)
	var pr content.Problems
	pr.Required("reason", reason, maxTextLength)
	if err := pr.Err(); err != nil {
		return err
	}
	var to recipient
	err := s.change(ctx, id, func(q *dbq.Queries, a dbq.LockApplicationRow) error {
		if !application.CanMove(a.Status, "rejected") {
			return application.ErrIllegalTransition
		}
		if err := closeOpenRequest(ctx, q, id, &p.UserID); err != nil {
			return err
		}
		if err := application.Move(ctx, q, id, a.Status, "rejected", &p.UserID, reason); err != nil {
			return err
		}
		var err error
		if to, err = s.recipient(ctx, q, a); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("application.rejected", id, map[string]any{"from": a.Status}))
	})
	if err != nil {
		return err
	}
	s.tell(ctx, notify.ApplicationRejected, id, to, map[string]string{"reason": reason})
	return nil
}

type OfferInput struct {
	Conditions string `json:"conditions"`
	// RespondBy is YYYY-MM-DD; empty means today plus
	// admissions.offer_response_days.
	RespondBy string `json:"respond_by"`
}

const letterDate = "2 January 2006"

// MakeOffer offers a place: it makes the offer letter from the template,
// keeps it privately, records the offer and tells the applicant.
func (s *Service) MakeOffer(ctx context.Context, p *auth.Principal, id uuid.UUID, in OfferInput, req audit.Request) error {
	if s.storage == nil {
		return ErrStorageOff
	}
	conditions := strings.TrimSpace(in.Conditions)
	var pr content.Problems
	pr.Optional("conditions", conditions, maxTextLength)
	q := dbq.New(s.pool)
	days, err := offerResponseDays(ctx, q)
	if err != nil {
		return err
	}
	today := s.today()
	respondBy := today.AddDate(0, 0, days)
	if v := strings.TrimSpace(in.RespondBy); v != "" {
		d, err := time.Parse(time.DateOnly, v)
		switch {
		case err != nil:
			pr.Add("respond_by", "Enter the date as YYYY-MM-DD.")
		case !d.After(today):
			pr.Add("respond_by", "Choose a date after today.")
		case d.After(today.AddDate(0, 0, maxOfferResponseDays)):
			pr.Add("respond_by", "Choose a date within a year from today.")
		default:
			respondBy = d
		}
	}
	if err := pr.Err(); err != nil {
		return err
	}

	// Check before making the letter; checked again under the lock.
	r, err := q.StaffGetApplication(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if r.Status != "under_review" {
		return application.ErrIllegalTransition
	}
	pdf, vars, err := s.offerLetter(ctx, q, r, conditions, respondBy)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("applications/%s/offer-letter-%s.pdf", id, uuid.NewString())
	stored := false
	var to recipient
	err = s.change(ctx, id, func(q *dbq.Queries, a dbq.LockApplicationRow) error {
		if a.Status != "under_review" {
			return application.ErrIllegalTransition
		}
		if err := s.storage.PutPrivate(ctx, s.bucket, key, pdf, "application/pdf"); err != nil {
			return fmt.Errorf("store offer letter: %w", err)
		}
		stored = true
		sum := sha256.Sum256(pdf)
		fileID, err := q.InsertPrivateFile(ctx, dbq.InsertPrivateFileParams{
			Bucket: s.bucket, ObjectKey: key, OriginalName: "Offer letter " + r.Reference + ".pdf", ContentType: "application/pdf",
			SizeBytes: int64(len(pdf)), Sha256: sum[:], ScanStatus: "not_scanned", UploadedByUserID: &p.UserID,
		})
		if err != nil {
			return err
		}
		o, err := q.InsertOffer(ctx, dbq.InsertOfferParams{ApplicationID: id, LetterFileID: &fileID, Conditions: conditions,
			IssuedBy: p.UserID, RespondBy: respondBy})
		if err != nil {
			return err
		}
		if err := application.Move(ctx, q, id, a.Status, "offered", &p.UserID,
			"Place offered; please reply by "+respondBy.Format(letterDate)); err != nil {
			return err
		}
		if to, err = s.recipient(ctx, q, a); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("application.offered", id, map[string]any{
			"offer_id": o.ID.String(), "respond_by": respondBy.Format(time.DateOnly), "conditions": conditions != ""}))
	})
	if err != nil {
		if stored {
			if derr := s.storage.Delete(context.WithoutCancel(ctx), s.bucket, key); derr != nil {
				s.logger.Error("could not remove unused offer letter", slog.String("key", key), slog.String("error", derr.Error()))
			}
		}
		return err
	}
	s.tell(ctx, notify.ApplicationOffered, id, to, map[string]string{"intake": vars["intake"], "respond_by": vars["respond_by"]})
	return nil
}

// offerLetter renders the letter for an application under review.
func (s *Service) offerLetter(ctx context.Context, q *dbq.Queries, r dbq.StaffGetApplicationRow, conditions string, respondBy time.Time) ([]byte, map[string]string, error) {
	ap, err := q.GetApplicantForLetter(ctx, r.ApplicantID)
	if err != nil {
		return nil, nil, err
	}
	prog, err := q.GetProgrammeForLetter(ctx, dbq.GetProgrammeForLetterParams{ID: r.ProgrammeID, IntakeID: r.IntakeID})
	if err != nil {
		return nil, nil, err
	}
	feeRows, err := q.ListLetterFees(ctx, dbq.ListLetterFeesParams{ProgrammeID: r.ProgrammeID, IntakeID: r.IntakeID})
	if err != nil {
		return nil, nil, err
	}
	vars := map[string]string{
		"name": ap.FirstName, "reference": r.Reference, "programme": prog.Title,
		"intake": "next", "starts_on": "the date the college confirms", "respond_by": respondBy.Format(letterDate),
	}
	if prog.IntakeName != nil {
		vars["intake"] = *prog.IntakeName
	}
	if prog.IntakeStartsOn != nil {
		vars["starts_on"] = prog.IntakeStartsOn.Format(letterDate)
	}
	heading, body, err := s.notify.Letter(ctx, notify.OfferLetter, vars)
	if err != nil {
		return nil, nil, err
	}
	fees := make([]letters.Fee, 0, len(feeRows))
	for _, f := range feeRows {
		fees = append(fees, letters.Fee{Label: f.Label, Frequency: f.Frequency, AmountCents: f.AmountCents, Notes: f.Notes})
	}
	pdf, err := letters.OfferLetter(letters.Offer{
		Date: s.now().In(nairobi), Reference: r.Reference, Name: fullName(ap.FirstName, ap.MiddleName, ap.LastName), Email: ap.Email,
		Heading: heading, Body: body, Conditions: conditions, Fees: fees, Sample: prog.IsSample,
	})
	return pdf, vars, err
}

// OfferLetterLink gives staff a short-lived link to the offer letter. Audited.
func (s *Service) OfferLetterLink(ctx context.Context, p *auth.Principal, id uuid.UUID, req audit.Request) (*application.FileLink, error) {
	var link *application.FileLink
	err := s.tx(ctx, func(q *dbq.Queries) error {
		o, err := q.GetOffer(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && o.LetterFileID == nil) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if link, err = application.IssueLink(ctx, q, *o.LetterFileID, p.UserID, s.now()); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, entry("file.link_issued", id, map[string]any{"file": "offer_letter"}))
	})
	return link, err
}

// DefaultStudentNumberFormat applies while student_number.format is empty (DQ-08).
const DefaultStudentNumberFormat = "DTC/{YYYY}/{NNNN}"

// FormatStudentNumber fills {YYYY}, {YY} and {NNNN} (at least four digits).
func FormatStudentNumber(format string, year int, n int64) string {
	return strings.NewReplacer(
		"{YYYY}", strconv.Itoa(year),
		"{YY}", fmt.Sprintf("%02d", year%100),
		"{NNNN}", fmt.Sprintf("%04d", n),
	).Replace(format)
}

// Enrol turns an applicant who accepted their offer into a student: student
// number, student record, the applicant's account becomes a student account
// (signed out everywhere; sign-in by student number comes in Stage 12), and
// an empty draft for the first invoice, which Finance completes (Stage 14).
// All in one transaction.
func (s *Service) Enrol(ctx context.Context, p *auth.Principal, id uuid.UUID, req audit.Request) (*Student, error) {
	var out Student
	var to recipient
	err := s.change(ctx, id, func(q *dbq.Queries, a dbq.LockApplicationRow) error {
		if a.Status != "offer_accepted" {
			return application.ErrIllegalTransition
		}
		ap, err := q.GetApplicantForEnrolment(ctx, a.ApplicantID)
		if err != nil {
			return err
		}
		if ap.UserID == nil {
			return ErrNotApplicantAccount
		}
		if n, err := q.MakeStudentAccount(ctx, *ap.UserID); err != nil || n != 1 {
			if err == nil {
				err = ErrNotApplicantAccount
			}
			return err
		}
		role, err := q.GetRoleByKey(ctx, "student")
		if err != nil {
			return err
		}
		if err := q.AssignRole(ctx, dbq.AssignRoleParams{UserID: *ap.UserID, RoleID: role.ID, GrantedBy: &p.UserID}); err != nil {
			return err
		}
		if _, err := q.RevokeUserSessions(ctx, *ap.UserID); err != nil {
			return err
		}

		prog, err := q.GetProgrammeForLetter(ctx, dbq.GetProgrammeForLetterParams{ID: a.ProgrammeID, IntakeID: a.IntakeID})
		if err != nil {
			return err
		}
		year := s.now().In(nairobi).Year()
		if prog.IntakeStartsOn != nil {
			year = prog.IntakeStartsOn.Year()
		}
		format := DefaultStudentNumberFormat
		if rows, err := q.GetSiteSettings(ctx, []string{"student_number.format"}); err != nil {
			return err
		} else if len(rows) == 1 && strings.Contains(rows[0].Value, "{NNNN}") {
			format = strings.TrimSpace(rows[0].Value)
		}
		// Numbers restart each year only when the year is part of them.
		period := int32(0)
		if strings.Contains(format, "{YYYY}") || strings.Contains(format, "{YY}") {
			period = int32(year)
		}
		seq, err := q.NextCounterValue(ctx, dbq.NextCounterValueParams{Name: "student", Period: period})
		if err != nil {
			return err
		}
		number := FormatStudentNumber(format, year, seq)

		st, err := q.InsertStudent(ctx, dbq.InsertStudentParams{
			StudentNumber: number, UserID: *ap.UserID, ApplicantID: ap.ID, ApplicationID: id, ProgrammeID: a.ProgrammeID, IntakeID: a.IntakeID,
			FirstName: ap.FirstName, MiddleName: ap.MiddleName, LastName: ap.LastName, DateOfBirth: ap.DateOfBirth, Email: ap.Email, Phone: ap.Phone,
			IdentityDocumentType: ap.IdentityDocumentType, IdentityDocument: ap.IdentityDocument, EnrolledBy: p.UserID,
		})
		if err != nil {
			return err
		}
		inv, err := q.InsertDraftInvoice(ctx, dbq.InsertDraftInvoiceParams{StudentID: st.ID, IntakeID: a.IntakeID, CreatedBy: p.UserID,
			Note: "Opened at enrolment. Finance adds the fee lines and issues it."})
		if err != nil {
			return err
		}
		if err := application.Move(ctx, q, id, a.Status, "enrolled", &p.UserID, "Enrolled as student "+number); err != nil {
			return err
		}
		out = Student{ID: st.ID, StudentNumber: number, Status: "active", EnrolledAt: st.EnrolledAt, InvoiceID: inv, InvoiceStatus: "draft"}
		if to, err = s.recipient(ctx, q, a); err != nil {
			return err
		}
		if err := audit.Write(ctx, q, req, entry("application.enrolled", id, map[string]any{"student_id": st.ID.String(), "student_number": number})); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "student.created", EntityType: "student", EntityID: st.ID.String(),
			Details: map[string]any{"student_number": number, "application_id": id.String(), "invoice_id": inv.String()}})
	})
	if err != nil {
		return nil, err
	}
	s.tell(ctx, notify.ApplicationEnrolled, id, to, map[string]string{"student_number": out.StudentNumber})
	return &out, nil
}

// recipient is who to tell about a decision, read inside the transaction.
type recipient struct {
	name, email, programme, reference string
	phone                             *string
}

func (s *Service) recipient(ctx context.Context, q *dbq.Queries, a dbq.LockApplicationRow) (recipient, error) {
	ap, err := q.GetApplicantForLetter(ctx, a.ApplicantID)
	if err != nil {
		return recipient{}, err
	}
	prog, err := q.GetProgrammeForLetter(ctx, dbq.GetProgrammeForLetterParams{ID: a.ProgrammeID, IntakeID: a.IntakeID})
	if err != nil {
		return recipient{}, err
	}
	return recipient{name: ap.FirstName, email: ap.Email, phone: ap.Phone, programme: prog.Title, reference: a.Reference}, nil
}

// tell sends the email (and records the SMS as skipped until DG-03). A
// failure is logged; it never undoes the decision.
func (s *Service) tell(ctx context.Context, template string, id uuid.UUID, to recipient, vars map[string]string) {
	all := map[string]string{
		"name": to.name, "reference": to.reference, "programme": to.programme,
		"link": s.appBaseURL + "/apply/application?id=" + id.String(),
	}
	for k, v := range vars {
		all[k] = v
	}
	m := notify.Message{Template: template, To: to.email, Vars: all, RelatedType: "application", RelatedID: id.String()}
	if err := s.notify.Email(ctx, m); err != nil {
		s.logger.Error("admissions email not queued", slog.String("template", template), slog.String("error", err.Error()))
	}
	if to.phone != nil && *to.phone != "" {
		m.To = *to.phone
		if err := s.notify.SMS(ctx, m); err != nil {
			s.logger.Error("admissions SMS not recorded", slog.String("template", template), slog.String("error", err.Error()))
		}
	}
}
