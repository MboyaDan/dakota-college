// Package kcb receives KCB Buni Instant Payment Notifications for the
// college account (Stage 15b, brief 9). KCB POSTs a notification after a
// payment is credited; the payer's reference should be the student number.
//
// A notification is accepted only on a path holding a long secret (and, if
// configured, from listed addresses), stored as received with its Signature
// header, and recorded once per KCB transaction reference.
//
// KCB has not yet supplied how to verify the Signature header, so a
// notification is not proof of payment (D-175): a match becomes a pending
// bank payment that Finance confirms against the KCB statement, exactly as
// a bank slip would. Anything else waits for Finance to assign or return.
package kcb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/callbackguard"
	"github.com/MboyaDan/dakota-college/backend/internal/config"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/finance"
)

var (
	ErrUnauthorised = callbackguard.ErrUnauthorised
	ErrThrottled    = callbackguard.ErrThrottled
	ErrNotFound     = errors.New("not found")
	ErrDecided      = errors.New("transaction already settled")
)

var nairobi = time.FixedZone("EAT", 3*60*60)

type Service struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
	cfg    config.KcbIPNConfig
	now    func() time.Time
	guard  *callbackguard.Guard
}

type Options struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger
	Config config.KcbIPNConfig
	Now    func() time.Time
}

// NewService returns nil when KCB IPN is not enabled.
func NewService(o Options) *Service {
	if !o.Config.Enabled {
		return nil
	}
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Service{pool: o.Pool, logger: o.Logger, cfg: o.Config, now: now,
		guard: callbackguard.New(o.Config.Secret, o.Config.AllowedIPs, func() time.Time { return now() })}
}

func (s *Service) tx(ctx context.Context, fn func(q *dbq.Queries) error) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error { return fn(dbq.New(tx)) })
}

// NotificationPath is where KCB sends notifications, relative to the API host.
func NotificationPath(secret string) string {
	return "/api/v1/payments/kcb/" + secret + "/notification"
}

// Notification is what KCB sends (IPN specification).
type Notification struct {
	TransactionReference string `json:"transactionReference"`
	RequestID            string `json:"requestId"`
	ChannelCode          string `json:"channelCode"`
	Timestamp            string `json:"timestamp"`
	TransactionAmount    string `json:"transactionAmount"`
	Currency             string `json:"currency"`
	CustomerReference    string `json:"customerReference"`
	CustomerName         string `json:"customerName"`
	CustomerMobileNumber string `json:"customerMobileNumber"`
	Balance              string `json:"balance"`
	Narration            string `json:"narration"`
	CreditAccount        string `json:"creditAccountIdentifier"`
	OrgShortCode         string `json:"organizationShortCode"`
	TillNumber           string `json:"tillNumber"`
}

// Answer is the acknowledgement KCB expects.
type Answer struct {
	TransactionID string `json:"transactionID"`
	StatusCode    string `json:"statusCode"`
	StatusMessage string `json:"statusMessage"`
}

var reference = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// sentAt reads KCB's timestamp (yyyyMMddHHmmss, Nairobi). The format is not
// confirmed by KCB (their sample has 13 digits), so anything else falls back
// to when it arrived; the text is kept as sent.
func sentAt(raw string, received time.Time) time.Time {
	if t, err := time.ParseInLocation("20060102150405", strings.TrimSpace(raw), nairobi); err == nil {
		return t
	}
	return received
}

// accountKey is how a reference is compared with student numbers: capitals
// and digits only.
func accountKey(ref string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(ref) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func dateOf(t time.Time) time.Time {
	y, m, d := t.In(nairobi).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Receive handles one notification: guard, store, process. It returns the
// acknowledgement for KCB. A notification that is stored but cannot be
// read is acknowledged with statusCode 1 so KCB knows.
func (s *Service) Receive(ctx context.Context, secret string, ip *netip.Addr, signature string, body []byte) (*Answer, error) {
	if key, err := s.guard.Check(secret, ip); err != nil {
		if errors.Is(err, ErrUnauthorised) {
			s.logger.Warn("KCB notification refused", slog.String("client_ip", key))
		}
		return nil, err
	}
	if !json.Valid(body) {
		body, _ = json.Marshal(map[string]string{"unreadable": string(body)})
	}
	remote := ""
	if ip != nil {
		remote = ip.String()
	}
	if len(signature) > 4096 {
		signature = signature[:4096]
	}
	id, err := dbq.New(s.pool).InsertKcbNotification(ctx, dbq.InsertKcbNotificationParams{RemoteIp: remote, Signature: signature, Body: body})
	if err != nil {
		return nil, err
	}
	var n Notification
	_ = json.Unmarshal(body, &n)
	if err := s.process(ctx, id, body); err != nil {
		s.logger.Error("KCB notification stored but not processed", slog.Int64("notification", id), slog.String("error", err.Error()))
		return &Answer{TransactionID: n.TransactionReference, StatusCode: "1", StatusMessage: "Notification stored but could not be read"}, nil
	}
	return &Answer{TransactionID: n.TransactionReference, StatusCode: "0", StatusMessage: "Notification received"}, nil
}

// process records one stored notification, in one transaction.
func (s *Service) process(ctx context.Context, notification int64, body []byte) error {
	var n Notification
	if err := json.Unmarshal(body, &n); err != nil {
		return fmt.Errorf("read notification: %w", err)
	}
	n.TransactionReference = strings.TrimSpace(n.TransactionReference)
	amount, ok := finance.ParseAmount(n.TransactionAmount)
	if !reference.MatchString(n.TransactionReference) || !ok {
		return errors.New("notification without a valid reference or amount")
	}
	at := sentAt(n.Timestamp, s.now())
	req := audit.Request{RequestID: fmt.Sprintf("kcb-notification-%d", notification)}
	return s.tx(ctx, func(q *dbq.Queries) error {
		id, err := q.InsertKcbTransaction(ctx, dbq.InsertKcbTransactionParams{TransactionReference: n.TransactionReference,
			RequestID: n.RequestID, ChannelCode: n.ChannelCode, SentAt: at, SentAtRaw: n.Timestamp, AmountCents: amount,
			Currency: strings.ToUpper(strings.TrimSpace(n.Currency)), CustomerReference: strings.TrimSpace(n.CustomerReference),
			CustomerName: strings.TrimSpace(n.CustomerName), CustomerMobile: n.CustomerMobileNumber, Narration: n.Narration,
			CreditAccount: n.CreditAccount, OrgShortCode: n.OrgShortCode, TillNumber: n.TillNumber, NotificationID: notification,
			// No way to check the signature yet (D-175).
			Verified: false})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil // the same payment told twice: recorded the first time
		}
		if err != nil {
			return err
		}
		settle := func(status, reason string, student, payment *uuid.UUID) error {
			if err := q.SettleKcbTransaction(ctx, dbq.SettleKcbTransactionParams{ID: id, Status: status, Reason: reason,
				StudentID: student, PaymentID: payment}); err != nil {
				return err
			}
			return audit.Write(ctx, q, req, audit.Entry{Action: "finance.kcb_received", EntityType: "kcb_transaction", EntityID: id.String(),
				Details: map[string]any{"reference": n.TransactionReference, "amount_cents": amount, "status": status}})
		}
		leave := func(reason string) error { return settle("unmatched", reason, nil, nil) }

		switch c := s.cfg; {
		case strings.ToUpper(strings.TrimSpace(n.Currency)) != "KES":
			return settle("rejected", "Not in KES ("+n.Currency+"); only shillings are accepted.", nil, nil)
		case c.CreditAccount != "" && strings.TrimSpace(n.CreditAccount) != c.CreditAccount:
			return settle("rejected", "For account "+n.CreditAccount+", not the college's.", nil, nil)
		case c.Till != "" && strings.TrimSpace(n.TillNumber) != c.Till:
			return settle("rejected", "For till "+n.TillNumber+", not the college's.", nil, nil)
		case c.OrgShortCode != "" && strings.TrimSpace(n.OrgShortCode) != c.OrgShortCode:
			return settle("rejected", "For organisation "+n.OrgShortCode+", not the college's.", nil, nil)
		}

		if _, err := q.BankPaymentByReference(ctx, n.TransactionReference); err == nil {
			return leave("A bank payment with this reference was recorded by hand. Check it before assigning.")
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		students, err := q.StudentsByAccountKey(ctx, accountKey(n.CustomerReference))
		if err != nil {
			return err
		}
		switch {
		case len(students) > 1:
			return leave(fmt.Sprintf("The reference %q matches more than one student.", n.CustomerReference))
		case len(students) == 0:
			return leave(fmt.Sprintf("No student has the reference %q.", n.CustomerReference))
		case s.cfg.AutoCreditMaxCents > 0 && amount > s.cfg.AutoCreditMaxCents:
			return leave("Above the amount matched automatically. Find it on the KCB statement, then assign it.")
		}
		student := students[0]
		payment, err := q.InsertBankNotificationPayment(ctx, dbq.InsertBankNotificationPaymentParams{StudentID: student, AmountCents: amount,
			PaidOn: dateOf(at), Reference: n.TransactionReference, PayerName: strings.TrimSpace(n.CustomerName),
			Note: "KCB notification: confirm it on the KCB statement."})
		if err != nil {
			return err
		}
		// Pending until Finance confirms it: the notification is not yet
		// verifiable (D-175).
		return settle("matched", "", &student, &payment)
	})
}
