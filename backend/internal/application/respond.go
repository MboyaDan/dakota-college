package application

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// The applicant's side of the admissions workflow (Stage 10a): reply to a
// request for more information, accept an offer, or withdraw. Each is one
// transaction under the application's row lock, so a double tap or a
// simultaneous staff action changes the status once.

const MaxReplyLength = 2000

// ErrOfferExpired means the respond-by date has passed; the offer lapsed.
var ErrOfferExpired = errors.New("the offer has expired")

// OfferView is the offer as the applicant sees it.
type OfferView struct {
	IssuedAt   time.Time  `json:"issued_at"`
	RespondBy  string     `json:"respond_by"`
	Conditions string     `json:"conditions"`
	AcceptedAt *time.Time `json:"accepted_at"`
	DeclinedAt *time.Time `json:"declined_at"`
	HasLetter  bool       `json:"has_letter"`
}

// InfoRequestView is what the admissions office asked for and is waiting on.
type InfoRequestView struct {
	Message     string    `json:"message"`
	RequestedAt time.Time `json:"requested_at"`
}

// withdrawable: an applicant can withdraw until enrolled or decided.
func withdrawable(status string) bool { return CanMove(status, "withdrawn") }

// addWorkflow fills the offer, the open information request and what the
// applicant may do now.
func (s *Service) addWorkflow(ctx context.Context, q *dbq.Queries, d *Detail) error {
	d.CanWithdraw = withdrawable(d.Status)
	d.CanUpload = d.Status == "draft" || d.Status == "info_requested"
	o, err := q.GetOffer(ctx, d.ID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return err
	default:
		d.Offer = &OfferView{IssuedAt: o.IssuedAt, RespondBy: o.RespondBy.Format(time.DateOnly), Conditions: o.Conditions,
			AcceptedAt: o.AcceptedAt, DeclinedAt: o.DeclinedAt, HasLetter: o.LetterFileID != nil}
	}
	if d.Status == "info_requested" {
		r, err := q.GetOpenInfoRequest(ctx, d.ID)
		if err != nil {
			return err
		}
		d.InfoRequest = &InfoRequestView{Message: r.Message, RequestedAt: r.RequestedAt}
	}
	return nil
}

// act locks the applicant's own application and runs fn in one transaction.
// An offer past its respond-by date lapses first, and fn is not run.
func (s *Service) act(ctx context.Context, userID, id uuid.UUID, fn func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error) error {
	lapsed := false
	err := s.tx(ctx, func(q *dbq.Queries) error {
		a, err := q.LockOwnApplication(ctx, dbq.LockOwnApplicationParams{ID: id, UserID: &userID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if a.Status == "offered" {
			o, err := q.GetOffer(ctx, a.ID)
			if err != nil {
				return err
			}
			if o.RespondBy.Before(s.today()) {
				lapsed = true
				return Move(ctx, q, a.ID, "offered", "expired", nil, "Offer not accepted by its respond-by date")
			}
		}
		return fn(q, a)
	})
	if err == nil && lapsed {
		return ErrOfferExpired
	}
	return err
}

// Reply answers an open request for more information and sends the
// application back to the admissions office for review.
func (s *Service) Reply(ctx context.Context, userID, id uuid.UUID, message string, req audit.Request) error {
	message = strings.TrimSpace(message)
	var p content.Problems
	p.Required("message", message, MaxReplyLength)
	if err := p.Err(); err != nil {
		return err
	}
	return s.act(ctx, userID, id, func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error {
		if a.Status != "info_requested" {
			return ErrIllegalTransition
		}
		r, err := q.GetOpenInfoRequest(ctx, a.ID)
		if err != nil {
			return err
		}
		if _, err := q.CloseInfoRequest(ctx, dbq.CloseInfoRequestParams{ID: r.ID, Response: message}); err != nil {
			return err
		}
		if err := Move(ctx, q, a.ID, a.Status, "under_review", nil, "Applicant replied"); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "application.info_replied", EntityType: "application", EntityID: a.ID.String(),
			Details: map[string]any{"request_id": r.ID.String(), "length": utf8.RuneCountInString(message)}})
	})
}

// AcceptOffer accepts the offer on the applicant's own application.
func (s *Service) AcceptOffer(ctx context.Context, userID, id uuid.UUID, req audit.Request) error {
	return s.act(ctx, userID, id, func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error {
		if a.Status != "offered" {
			return ErrIllegalTransition
		}
		if n, err := q.AcceptOffer(ctx, a.ID); err != nil || n != 1 {
			if err == nil {
				err = ErrIllegalTransition
			}
			return err
		}
		if err := Move(ctx, q, a.ID, a.Status, "offer_accepted", nil, "Applicant accepted the offer"); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "application.offer_accepted", EntityType: "application", EntityID: a.ID.String()})
	})
}

// Withdraw ends the application at the applicant's request. Withdrawing
// while an offer is open declines it. The reason is optional.
func (s *Service) Withdraw(ctx context.Context, userID, id uuid.UUID, reason string, req audit.Request) error {
	reason = strings.TrimSpace(reason)
	var p content.Problems
	p.Optional("reason", reason, MaxReplyLength)
	if err := p.Err(); err != nil {
		return err
	}
	return s.act(ctx, userID, id, func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error {
		if !withdrawable(a.Status) {
			return ErrIllegalTransition
		}
		why := "Withdrawn by the applicant"
		if a.Status == "offered" {
			why = "Applicant declined the offer"
			if _, err := q.DeclineOffer(ctx, a.ID); err != nil {
				return err
			}
		}
		if reason != "" {
			why += ": " + reason
		}
		if a.Status == "info_requested" {
			r, err := q.GetOpenInfoRequest(ctx, a.ID)
			if err != nil {
				return err
			}
			if _, err := q.CloseInfoRequest(ctx, dbq.CloseInfoRequestParams{ID: r.ID}); err != nil {
				return err
			}
		}
		if err := Move(ctx, q, a.ID, a.Status, "withdrawn", nil, why); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "application.withdrawn", EntityType: "application", EntityID: a.ID.String(),
			Details: map[string]any{"from": a.Status}})
	})
}

// OfferLetterLink gives the applicant a short-lived link to their offer letter.
func (s *Service) OfferLetterLink(ctx context.Context, userID, id uuid.UUID) (*FileLink, error) {
	if _, err := s.load(ctx, userID, id); err != nil {
		return nil, err
	}
	q := dbq.New(s.pool)
	o, err := q.GetOffer(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && o.LetterFileID == nil) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.issueLink(ctx, q, *o.LetterFileID, userID)
}
