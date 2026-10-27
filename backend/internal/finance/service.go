// Package finance is fees and payments (brief 9, Stage 14a): fee structures,
// invoices with instalments, payments that wait for confirmation, receipts
// with gapless numbers, compensating adjustments and statements.
//
// Every balance is derived from records (NN-03, AC-10). A payment counts
// only once confirmed by a finance officer or, from Stage 15, by the
// verified M-Pesa flow (AC-11, NN-10). Issued invoices, decided payments,
// receipts and adjustments never change (NN-05); the database refuses it.
//
// Finance access is college-wide: finance.view reads every student's
// account (D-158).
package finance

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

const (
	PermView    = "finance.view"
	PermInvoice = "finance.invoice"
	PermRecord  = "finance.payment.record"
	PermConfirm = "finance.payment.confirm"
	PermAdjust  = "finance.adjust"
)

var (
	ErrNotFound = errors.New("not found")
	// ErrNotDraft: only a draft invoice can change, be issued or voided.
	ErrNotDraft = errors.New("invoice is not a draft")
	// ErrDecided: the payment was already confirmed or rejected.
	ErrDecided = errors.New("payment already decided")
	// ErrKeyReused: the idempotency key was used for a different request.
	ErrKeyReused = errors.New("idempotency key reused for another request")
	ErrNoStudent = errors.New("no student record")
)

type Service struct {
	pool    *pgxpool.Pool
	logger  *slog.Logger
	now     func() time.Time
	paybill string
}

type Options struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	Now    func() time.Time
	// MpesaPaybill is the configured paybill, shown to students; empty when
	// M-Pesa is not enabled.
	MpesaPaybill string
}

func NewService(o Options) *Service {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{pool: o.Pool, logger: o.Logger, now: now, paybill: o.MpesaPaybill}
}

func (s *Service) tx(ctx context.Context, fn func(q *dbq.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(dbq.New(tx)) })
}

// today is the calendar date in Nairobi, as date columns are read.
func (s *Service) today() time.Time { return dateOf(s.now()) }

func fullName(first, middle, last string) string {
	return strings.Join(strings.Fields(first+" "+middle+" "+last), " ")
}

// isSample: test students from the dev seed, whose documents say so.
func isSample(number string) bool { return strings.HasPrefix(number, "DTC/SAMPLE/") }

func isUniqueViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23505"
}

func tidy(v string, max int) string {
	v = strings.TrimSpace(v)
	if r := []rune(v); len(r) > max {
		v = string(r[:max])
	}
	return v
}

// ---------- idempotency (NN-04) ----------

const keyTTL = 24 * time.Hour

// Answer is the reply to a keyed request: the first answer, or the stored
// copy of it when the request is retried with the same key.
type Answer struct {
	Body     json.RawMessage
	Replayed bool
}

// canonical encodes v the way jsonb gives it back (keys sorted), so a
// replay is byte for byte the first answer.
func canonical(v any) ([]byte, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		return nil, err
	}
	return json.Marshal(tree)
}

// keyed runs fn once per key. The key is claimed, fn runs, and the answer
// is stored, all in one transaction: if fn fails nothing is kept and the
// same key may be tried again. A retry with the same key and request gets
// the stored answer; the same key with a different request is refused.
func (s *Service) keyed(ctx context.Context, scope, key string, request any, fn func(q *dbq.Queries) (any, error)) (*Answer, error) {
	if len(key) < 8 || len(key) > 128 {
		return nil, content.Problems{{Field: "idempotency_key", Message: "Reload the page and try again."}}.Err()
	}
	raw, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(append([]byte(scope+"|"), raw...))
	var out *Answer
	err = s.tx(ctx, func(q *dbq.Queries) error {
		n, err := q.ClaimIdempotencyKey(ctx, dbq.ClaimIdempotencyKeyParams{Scope: scope, Key: key, RequestHash: hash[:], ExpiresAt: s.now().Add(keyTTL)})
		if err != nil {
			return err
		}
		if n == 0 {
			rec, err := q.GetIdempotencyRecord(ctx, dbq.GetIdempotencyRecordParams{Scope: scope, Key: key})
			if err != nil {
				return err
			}
			if string(rec.RequestHash) != string(hash[:]) {
				return ErrKeyReused
			}
			if rec.CompletedAt == nil {
				return errors.New("idempotency record without a response")
			}
			body, err := canonical(json.RawMessage(rec.ResponseBody))
			if err != nil {
				return err
			}
			out = &Answer{Body: body, Replayed: true}
			return nil
		}
		v, err := fn(q)
		if err != nil {
			return err
		}
		body, err := canonical(v)
		if err != nil {
			return err
		}
		status := int32(200)
		if err := q.CompleteIdempotencyKey(ctx, dbq.CompleteIdempotencyKeyParams{Scope: scope, Key: key, ResponseStatus: &status, ResponseBody: body}); err != nil {
			return err
		}
		out = &Answer{Body: body}
		return nil
	})
	return out, err
}
