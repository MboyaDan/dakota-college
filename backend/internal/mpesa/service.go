// Package mpesa is the college paybill on Safaricom Daraja C2B (Stage 15a,
// brief 9, AC-11, DG-02). Students pay with their student number as the
// account number; Safaricom calls our confirmation URL for each payment.
//
// Daraja does not sign its callbacks, so a callback is accepted only on a
// path holding a long secret (and, if configured, from listed addresses).
// Every accepted callback is stored as received before it is processed.
// Each M-Pesa code is recorded once; a payment for our shortcode whose
// account number is a student's number is confirmed and receipted by
// itself; anything else waits for Finance (D-169 to D-172).
package mpesa

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/config"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/finance"
)

const PermSetup = "finance.mpesa.setup"

var (
	// ErrUnauthorised: not a callback we accept (wrong secret or address).
	ErrUnauthorised = errors.New("callback not accepted")
	ErrNotFound     = errors.New("not found")
	// ErrDecided: the transaction is no longer waiting for Finance.
	ErrDecided = errors.New("transaction already settled")
	// ErrSandboxOnly: simulating payments is for the sandbox.
	ErrSandboxOnly = errors.New("only in the sandbox")
)

var nairobi = time.FixedZone("EAT", 3*60*60)

type Service struct {
	pool    *pgxpool.Pool
	logger  *slog.Logger
	cfg     config.MpesaConfig
	daraja  *Daraja
	now     func() time.Time
	allowed []netip.Prefix
}

type Options struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	Config config.MpesaConfig
	// HTTPClient talks to Daraja; nil uses a client with a 30 second timeout.
	HTTPClient *http.Client
	Now        func() time.Time
}

// NewService returns nil when M-Pesa is not enabled.
func NewService(o Options) *Service {
	if !o.Config.Enabled {
		return nil
	}
	now := o.Now
	if now == nil {
		now = time.Now
	}
	client := o.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	s := &Service{pool: o.Pool, logger: o.Logger, cfg: o.Config, now: now,
		daraja: &Daraja{baseURL: o.Config.APIBaseURL, key: o.Config.ConsumerKey, secret: o.Config.ConsumerSecret,
			registerPath: o.Config.RegisterPath, simulPath: o.Config.SimulatePath, client: client, now: now}}
	for _, a := range o.Config.AllowedIPs {
		if p, err := netip.ParsePrefix(a); err == nil {
			s.allowed = append(s.allowed, p)
		} else if ip, err := netip.ParseAddr(a); err == nil {
			s.allowed = append(s.allowed, netip.PrefixFrom(ip, ip.BitLen()))
		}
	}
	return s
}

// ShortCode is the paybill students pay to.
func (s *Service) ShortCode() string { return s.cfg.ShortCode }

func (s *Service) tx(ctx context.Context, fn func(q *dbq.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(dbq.New(tx)) })
}

// CallbackPath is where Safaricom calls, relative to the API host.
func CallbackPath(secret, kind string) string {
	return "/api/v1/payments/c2b/" + secret + "/" + kind
}

// accepts checks the secret in the path and, if configured, the address.
func (s *Service) accepts(secret string, ip *netip.Addr) bool {
	if subtle.ConstantTimeCompare([]byte(secret), []byte(s.cfg.CallbackSecret)) != 1 {
		return false
	}
	if len(s.allowed) == 0 {
		return true
	}
	if ip == nil {
		return false
	}
	for _, p := range s.allowed {
		if p.Contains(ip.Unmap()) {
			return true
		}
	}
	return false
}

// Confirmation is what Safaricom sends for a completed payment.
type Confirmation struct {
	TransactionType   string          `json:"TransactionType"`
	TransID           string          `json:"TransID"`
	TransTime         string          `json:"TransTime"`
	TransAmount       json.RawMessage `json:"TransAmount"`
	BusinessShortCode string          `json:"BusinessShortCode"`
	BillRefNumber     string          `json:"BillRefNumber"`
	MSISDN            string          `json:"MSISDN"`
	FirstName         string          `json:"FirstName"`
	MiddleName        string          `json:"MiddleName"`
	LastName          string          `json:"LastName"`
}

var (
	transID = regexp.MustCompile(`^[A-Z0-9]{10}$`)
	amount  = regexp.MustCompile(`^(\d{1,10})(?:\.(\d{1,2}))?$`)
)

// amountCents reads "1234", "1234.5", "1234.50" or a JSON number into cents,
// on the digits, never through floating point.
func amountCents(raw json.RawMessage) (int64, bool) {
	m := amount.FindStringSubmatch(strings.Trim(strings.TrimSpace(string(raw)), `"`))
	if m == nil {
		return 0, false
	}
	whole, _ := strconv.ParseInt(m[1], 10, 64)
	frac := int64(0)
	if m[2] != "" {
		frac, _ = strconv.ParseInt((m[2] + "0")[:2], 10, 64)
	}
	c := whole*100 + frac
	return c, c > 0
}

// accountKey is how an account number is compared: capitals and digits only.
func accountKey(ref string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(ref) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (c Confirmation) payer() string {
	return strings.Join(strings.Fields(c.FirstName+" "+c.MiddleName+" "+c.LastName), " ")
}

func (c Confirmation) time() time.Time {
	t, err := time.ParseInLocation("20060102150405", strings.TrimSpace(c.TransTime), nairobi)
	if err != nil {
		return time.Time{}
	}
	return t
}

// Callback handles a call to the validation or confirmation URL. It stores
// the body as received, then (for a confirmation) records the payment. The
// error is ErrUnauthorised when the call is not accepted; any later failure
// is logged and the stored callback can be processed again.
func (s *Service) Callback(ctx context.Context, kind, secret string, ip *netip.Addr, body []byte) error {
	if !s.accepts(secret, ip) {
		return ErrUnauthorised
	}
	if !json.Valid(body) {
		body, _ = json.Marshal(map[string]string{"unreadable": string(body)})
	}
	remote := ""
	if ip != nil {
		remote = ip.String()
	}
	id, err := dbq.New(s.pool).InsertMpesaCallback(ctx, dbq.InsertMpesaCallbackParams{Kind: kind, RemoteIp: remote, Body: body})
	if err != nil {
		return err
	}
	if kind != "confirmation" {
		return nil
	}
	if err := s.process(ctx, id, body); err != nil {
		s.logger.Error("M-Pesa confirmation stored but not processed", slog.Int64("callback", id), slog.String("error", err.Error()))
	}
	return nil
}

// process records one stored confirmation, in one transaction.
func (s *Service) process(ctx context.Context, callback int64, body []byte) error {
	var c Confirmation
	if err := json.Unmarshal(body, &c); err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	c.TransID = strings.ToUpper(strings.TrimSpace(c.TransID))
	amount, ok := amountCents(c.TransAmount)
	at := c.time()
	if !transID.MatchString(c.TransID) || !ok || at.IsZero() {
		return fmt.Errorf("confirmation without a valid code, amount or time")
	}
	req := audit.Request{RequestID: fmt.Sprintf("mpesa-callback-%d", callback)}
	return s.tx(ctx, func(q *dbq.Queries) error {
		id, err := q.InsertMpesaTransaction(ctx, dbq.InsertMpesaTransactionParams{TransID: c.TransID, TransType: c.TransactionType, TransTime: at,
			AmountCents: amount, ShortCode: c.BusinessShortCode, BillRef: strings.TrimSpace(c.BillRefNumber), Msisdn: c.MSISDN,
			PayerName: c.payer(), CallbackID: callback})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // the same payment told twice: recorded the first time
		}
		if err != nil {
			return err
		}
		settle := func(status, reason string, student, payment *uuid.UUID) error {
			if err := q.SettleMpesaTransaction(ctx, dbq.SettleMpesaTransactionParams{ID: id, Status: status, Reason: reason,
				StudentID: student, PaymentID: payment}); err != nil {
				return err
			}
			return audit.Write(ctx, q, req, audit.Entry{Action: "finance.mpesa_received", EntityType: "mpesa_transaction", EntityID: id.String(),
				Details: map[string]any{"trans_id": c.TransID, "amount_cents": amount, "status": status}})
		}

		if strings.TrimSpace(c.BusinessShortCode) != s.cfg.ShortCode {
			return settle("rejected", "Paid to shortcode "+c.BusinessShortCode+", not the college's.", nil, nil)
		}

		students, err := q.StudentsByAccountKey(ctx, accountKey(c.BillRefNumber))
		if err != nil {
			return err
		}
		var student *uuid.UUID
		if len(students) == 1 {
			student = &students[0]
		}

		// A payment recorded by hand with this code: confirm it if it is the
		// same money for the same student, otherwise leave it for Finance.
		existing, err := q.MpesaPaymentByReference(ctx, c.TransID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
		case err != nil:
			return err
		case existing.AmountCents != amount || (student != nil && existing.StudentID != *student):
			return s.leave(ctx, q, id, "A payment with this code was recorded by hand for another student or amount. Check it before assigning.", req, c.TransID, amount)
		case existing.Status == "confirmed":
			return settle("matched", "Already confirmed by hand.", &existing.StudentID, &existing.ID)
		default:
			if _, err := finance.ConfirmByProvider(ctx, q, existing.ID, nil, "Confirmed by M-Pesa.", s.now()); err != nil {
				return err
			}
			return settle("matched", "", &existing.StudentID, &existing.ID)
		}

		if student == nil {
			reason := fmt.Sprintf("No student has the account number %q.", c.BillRefNumber)
			if len(students) > 1 {
				reason = fmt.Sprintf("The account number %q matches more than one student.", c.BillRefNumber)
			}
			return s.leave(ctx, q, id, reason, req, c.TransID, amount)
		}
		payment, err := q.InsertProviderPayment(ctx, dbq.InsertProviderPaymentParams{StudentID: *student, AmountCents: amount,
			PaidOn: dateOf(at), Reference: c.TransID, PayerName: c.payer(), Note: "Paid to the college paybill."})
		if err != nil {
			return err
		}
		if _, err := finance.ConfirmByProvider(ctx, q, payment, nil, "Confirmed by M-Pesa.", s.now()); err != nil {
			return err
		}
		return settle("matched", "", student, &payment)
	})
}

// leave keeps a transaction unmatched with the reason, for Finance.
func (s *Service) leave(ctx context.Context, q *dbq.Queries, id uuid.UUID, reason string, req audit.Request, code string, amount int64) error {
	if err := q.SettleMpesaTransaction(ctx, dbq.SettleMpesaTransactionParams{ID: id, Status: "unmatched", Reason: reason}); err != nil {
		return err
	}
	return audit.Write(ctx, q, req, audit.Entry{Action: "finance.mpesa_received", EntityType: "mpesa_transaction", EntityID: id.String(),
		Details: map[string]any{"trans_id": code, "amount_cents": amount, "status": "unmatched"}})
}

func dateOf(t time.Time) time.Time {
	y, m, d := t.In(nairobi).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
