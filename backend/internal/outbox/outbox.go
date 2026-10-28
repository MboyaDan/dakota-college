// Package outbox is the queue every outgoing email and text message goes
// through (Stage 16a). A message is a row in notifications; a worker sends
// due messages and retries failures with growing waits, so a mail server
// outage delays messages instead of losing them.
//
// The message text is stored encrypted with MESSAGE_KEY (AES-256-GCM) only
// while it may still be sent, because some messages carry sign-in or
// password reset links. It is erased when the message is sent, and a failed
// message's copy is erased when it expires. The encryption is bound to the
// channel and recipient, so changing a row's recipient makes it unreadable
// instead of sending its text to someone else.
//
// Delivery is at least once: a worker that stops after sending but before
// recording it sends that message again when its claim runs out.
package outbox

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/textproto"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/mail"
)

// Staff-facing emails whose text is in code, not an editable template.
const (
	SystemPasswordReset   = "account.password_reset"
	SystemEnquiryReceived = "enquiry.received"
	SystemEnquiryAssigned = "enquiry.assigned"
)

// Retry waits after the first, second, ... failed attempt. A message is
// tried len(RetryDelays)+1 times, over about five and a half hours.
var RetryDelays = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute, time.Hour, 4 * time.Hour}

const (
	// DefaultValidFor is how long a message may wait to be sent, and how
	// long a failed message can be sent again by staff.
	DefaultValidFor = 7 * 24 * time.Hour
	// claimFor is how long a worker holds a message it is sending.
	claimFor   = 2 * time.Minute
	sendWithin = 30 * time.Second
	batchSize  = 10
	// KeySize is the length of MESSAGE_KEY in bytes.
	KeySize = 32
)

// ErrSMSUnavailable is recorded on text messages until DG-03 clears.
var ErrSMSUnavailable = errors.New("no SMS provider is set up yet (DG-03)")

// SMSSender sends one text message. None exists until an SMS provider is
// chosen (DG-03).
type SMSSender interface {
	SendSMS(ctx context.Context, to, text string) error
}

type Options struct {
	Pool   *pgxpool.Pool
	Mailer mail.Mailer
	// SMS is nil until DG-03: text messages are then recorded as skipped.
	SMS    SMSSender
	Key    []byte
	Logger *slog.Logger
	Now    func() time.Time
	// PollInterval is how often the worker looks for due messages; new
	// messages wake it at once. Default 5 seconds.
	PollInterval time.Duration
}

type Queue struct {
	pool   *pgxpool.Pool
	mailer mail.Mailer
	sms    SMSSender
	aead   cipher.AEAD
	logger *slog.Logger
	now    func() time.Time
	poll   time.Duration
	wake   chan struct{}
}

func New(o Options) (*Queue, error) {
	if len(o.Key) != KeySize {
		return nil, fmt.Errorf("message key must be %d bytes", KeySize)
	}
	block, err := aes.NewCipher(o.Key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	q := &Queue{pool: o.Pool, mailer: o.Mailer, sms: o.SMS, aead: aead, logger: o.Logger, now: o.Now, poll: o.PollInterval, wake: make(chan struct{}, 1)}
	if q.now == nil {
		q.now = time.Now
	}
	if q.poll <= 0 {
		q.poll = 5 * time.Second
	}
	return q, nil
}

// Item says what a message is and what it is about.
type Item struct {
	// Exactly one of TemplateKey (an editable template) and SystemKey
	// (fixed text in code) is set.
	TemplateKey string
	SystemKey   string
	RelatedType string
	RelatedID   string
	// ValidFor is how long the message may wait; messages with a link
	// that expires use the link's lifetime. Zero means DefaultValidFor.
	ValidFor time.Duration
}

// payload is what is stored encrypted.
type payload struct {
	Subject string `json:"s,omitempty"`
	Text    string `json:"t"`
	ReplyTo string `json:"r,omitempty"`
}

// Email queues an email and wakes the worker.
func (q *Queue) Email(ctx context.Context, it Item, m mail.Message) (uuid.UUID, error) {
	return q.insert(ctx, it, "email", m.To, payload{Subject: m.Subject, Text: m.Text, ReplyTo: m.ReplyTo})
}

// SMS queues a text message, or records it as skipped while no SMS
// provider is set up.
func (q *Queue) SMS(ctx context.Context, it Item, to, text string) (uuid.UUID, error) {
	if q.sms == nil {
		return dbq.New(q.pool).InsertNotification(ctx, dbq.InsertNotificationParams{
			TemplateKey: optional(it.TemplateKey), SystemKey: optional(it.SystemKey), Channel: "sms", Recipient: to,
			Status: "skipped", Error: ErrSMSUnavailable.Error(), RelatedType: it.RelatedType, RelatedID: it.RelatedID,
			NextAttemptAt: q.now(), ExpiresAt: q.now(),
		})
	}
	return q.insert(ctx, it, "sms", to, payload{Text: text})
}

// SMSEnabled reports whether text messages are sent.
func (q *Queue) SMSEnabled() bool { return q.sms != nil }

func (q *Queue) insert(ctx context.Context, it Item, channel, to string, p payload) (uuid.UUID, error) {
	if (it.TemplateKey == "") == (it.SystemKey == "") {
		return uuid.Nil, errors.New("outbox: set exactly one of TemplateKey and SystemKey")
	}
	sealed, err := q.seal(channel, to, p)
	if err != nil {
		return uuid.Nil, err
	}
	validFor := it.ValidFor
	if validFor <= 0 {
		validFor = DefaultValidFor
	}
	now := q.now()
	id, err := dbq.New(q.pool).InsertNotification(ctx, dbq.InsertNotificationParams{
		TemplateKey: optional(it.TemplateKey), SystemKey: optional(it.SystemKey), Channel: channel, Recipient: to,
		Status: "queued", RelatedType: it.RelatedType, RelatedID: it.RelatedID, Payload: sealed, NextAttemptAt: now, ExpiresAt: now.Add(validFor),
	})
	if err != nil {
		return uuid.Nil, err
	}
	q.Wake()
	return id, nil
}

// Wake asks the worker to look for due messages now.
func (q *Queue) Wake() {
	select {
	case q.wake <- struct{}{}:
	default:
	}
}

// Run sends due messages until ctx ends.
func (q *Queue) Run(ctx context.Context) {
	t := time.NewTicker(q.poll)
	defer t.Stop()
	for {
		if _, err := q.Process(ctx); err != nil && ctx.Err() == nil {
			q.logger.Error("message queue", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-q.wake:
		}
	}
}

// Process gives up expired messages, then sends every due message once. It
// returns how many it tried. Several workers can run it at the same time.
func (q *Queue) Process(ctx context.Context) (int, error) {
	db := dbq.New(q.pool)
	now := q.now()
	if _, err := db.ExpireNotifications(ctx, now); err != nil {
		return 0, err
	}
	if _, err := db.EraseExpiredPayloads(ctx, now); err != nil {
		return 0, err
	}
	tried := 0
	for {
		now = q.now()
		rows, err := db.ClaimNotifications(ctx, dbq.ClaimNotificationsParams{LockedUntil: now.Add(claimFor), Now: now, MaxRows: batchSize})
		if err != nil {
			return tried, err
		}
		for _, r := range rows {
			q.attempt(ctx, db, r)
		}
		tried += len(rows)
		if len(rows) < batchSize || ctx.Err() != nil {
			return tried, ctx.Err()
		}
	}
}

func (q *Queue) attempt(ctx context.Context, db *dbq.Queries, r dbq.ClaimNotificationsRow) {
	log := q.logger.With(slog.String("notification_id", r.ID.String()), slog.String("channel", r.Channel), slog.Int("attempt", int(r.Attempts)))
	p, err := q.open(r.Channel, r.Recipient, r.Payload)
	if err != nil {
		log.Error("stored message could not be read", slog.String("error", err.Error()))
		q.record(ctx, db, r, errors.New("the stored message could not be read (was MESSAGE_KEY changed?)"), true)
		return
	}
	sendCtx, cancel := context.WithTimeout(ctx, sendWithin)
	defer cancel()
	switch {
	case r.Channel == "email":
		err = q.mailer.Send(sendCtx, mail.Message{To: r.Recipient, Subject: p.Subject, Text: p.Text, ReplyTo: p.ReplyTo})
	case q.sms != nil:
		err = q.sms.SendSMS(sendCtx, r.Recipient, p.Text)
	default:
		err = ErrSMSUnavailable
	}
	if err != nil {
		log.Warn("message not sent", slog.String("error", err.Error()))
	}
	q.record(ctx, db, r, err, Permanent(err))
}

func (q *Queue) record(ctx context.Context, db *dbq.Queries, r dbq.ClaimNotificationsRow, sendErr error, permanent bool) {
	// The outcome is recorded even if ctx ended during the send.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	var err error
	switch {
	case sendErr == nil:
		err = db.MarkNotificationSent(ctx, dbq.MarkNotificationSentParams{ID: r.ID, Now: q.now()})
	case permanent || int(r.Attempts) > len(RetryDelays):
		err = db.MarkNotificationFailed(ctx, dbq.MarkNotificationFailedParams{ID: r.ID, Error: truncate(sendErr.Error(), 500)})
	default:
		err = db.RetryNotificationLater(ctx, dbq.RetryNotificationLaterParams{
			ID: r.ID, Error: truncate(sendErr.Error(), 500), NextAttemptAt: q.now().Add(RetryDelays[r.Attempts-1]),
		})
	}
	if err != nil {
		q.logger.Error("could not record message outcome", slog.String("notification_id", r.ID.String()), slog.String("error", err.Error()))
	}
}

// Permanent reports whether retrying cannot help: the address is invalid or
// the receiving server refused the mailbox. Other failures (the server is
// down, a sign-in problem on our side) are retried.
func Permanent(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, mail.ErrBadAddress) {
		return true
	}
	var te *textproto.Error
	if errors.As(err, &te) {
		return te.Code == 550 || te.Code == 551 || te.Code == 553
	}
	return false
}

func (q *Queue) seal(channel, to string, p payload) ([]byte, error) {
	plain, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, q.aead.NonceSize(), q.aead.NonceSize()+len(plain)+q.aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return q.aead.Seal(nonce, nonce, plain, boundTo(channel, to)), nil
}

func (q *Queue) open(channel, to string, sealed []byte) (payload, error) {
	var p payload
	n := q.aead.NonceSize()
	if len(sealed) < n {
		return p, errors.New("stored message too short")
	}
	plain, err := q.aead.Open(nil, sealed[:n], sealed[n:], boundTo(channel, to))
	if err != nil {
		return p, err
	}
	return p, json.Unmarshal(plain, &p)
}

func boundTo(channel, to string) []byte { return []byte(channel + "\x00" + to) }

func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
