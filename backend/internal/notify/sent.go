package notify

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/outbox"
)

// systemNames name the staff-facing emails whose text is in code.
var systemNames = map[string]string{
	outbox.SystemPasswordReset:   "Password reset (staff and students)",
	outbox.SystemEnquiryReceived: "New website enquiry (to the office)",
	outbox.SystemEnquiryAssigned: "Enquiry assigned (to staff)",
}

// Statuses a sent message can be filtered by.
var Statuses = []string{"queued", "sending", "sent", "failed", "skipped"}

const sentPageSize = 50

// Sent is one row of the message log. The text is never shown.
type Sent struct {
	ID            uuid.UUID  `json:"id"`
	Name          string     `json:"name"`
	Channel       string     `json:"channel"`
	Recipient     string     `json:"recipient"`
	Status        string     `json:"status"`
	Error         string     `json:"error"`
	Attempts      int32      `json:"attempts"`
	RelatedType   string     `json:"related_type"`
	RelatedID     string     `json:"related_id"`
	CreatedAt     time.Time  `json:"created_at"`
	SentAt        *time.Time `json:"sent_at"`
	NextAttemptAt *time.Time `json:"next_attempt_at"`
	CanResend     bool       `json:"can_resend"`
}

type SentFilter struct {
	Status  string
	Channel string
	Query   string
	Page    int
}

type SentPage struct {
	Items    []Sent `json:"items"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// ListSent returns the message log, newest first.
func (s *Service) ListSent(ctx context.Context, f SentFilter) (SentPage, error) {
	if f.Page < 1 {
		f.Page = 1
	}
	q := dbq.New(s.pool)
	status, channel, search := optional(f.Status), optional(f.Channel), likePattern(f.Query)
	total, err := q.CountNotifications(ctx, dbq.CountNotificationsParams{Status: status, Channel: channel, Search: search})
	if err != nil {
		return SentPage{}, err
	}
	rows, err := q.ListNotifications(ctx, dbq.ListNotificationsParams{
		Status: status, Channel: channel, Search: search, MaxRows: sentPageSize, SkipRows: int32((f.Page - 1) * sentPageSize),
	})
	if err != nil {
		return SentPage{}, err
	}
	now := time.Now()
	out := SentPage{Items: make([]Sent, 0, len(rows)), Total: total, Page: f.Page, PageSize: sentPageSize}
	for _, r := range rows {
		item := Sent{
			ID: r.ID, Channel: r.Channel, Recipient: r.Recipient, Status: r.Status, Error: r.Error, Attempts: r.Attempts,
			RelatedType: r.RelatedType, RelatedID: r.RelatedID, CreatedAt: r.CreatedAt, SentAt: r.SentAt,
			CanResend: r.Status == "failed" && r.HasPayload && r.ExpiresAt.After(now),
		}
		switch {
		case r.TemplateName != nil:
			item.Name = *r.TemplateName
		case r.SystemKey != nil:
			item.Name = systemNames[*r.SystemKey]
		}
		if r.Status == "queued" && r.Attempts > 0 {
			item.NextAttemptAt = &r.NextAttemptAt
		}
		out.Items = append(out.Items, item)
	}
	return out, nil
}

// ErrNotResendable means the message is not a failed one whose text is
// still stored.
var ErrNotResendable = errors.New("message cannot be sent again")

// Resend queues a failed message again for a fresh set of attempts.
func (s *Service) Resend(ctx context.Context, id uuid.UUID, req audit.Request) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		r, err := q.ResendNotification(ctx, dbq.ResendNotificationParams{ID: id, Now: time.Now()})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotResendable
		}
		if err != nil {
			return err
		}
		key := r.TemplateKey
		if key == nil {
			key = r.SystemKey
		}
		return audit.Write(ctx, q, req, audit.Entry{
			Action: "notifications.resent", EntityType: "notification", EntityID: id.String(),
			Details: map[string]any{"message": *key, "channel": r.Channel},
		})
	})
	if err == nil {
		s.queue.Wake()
	}
	return err
}

// SentCounts are the numbers on the staff overview.
type SentCounts struct {
	Waiting int64 `json:"waiting"`
	// FailedRecent counts messages that failed in the last seven days.
	FailedRecent int64 `json:"failed_recent"`
}

func (s *Service) SentCounts(ctx context.Context) (SentCounts, error) {
	r, err := dbq.New(s.pool).CountNotificationsByStatus(ctx, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		return SentCounts{}, err
	}
	return SentCounts{Waiting: r.Waiting, FailedRecent: r.FailedRecent}, nil
}

func optional(s string) *string {
	if s = strings.TrimSpace(s); s == "" {
		return nil
	}
	return &s
}

// likePattern escapes LIKE wildcards in a search box entry.
func likePattern(q string) *string {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil
	}
	if len([]rune(q)) > 80 {
		q = string([]rune(q)[:80])
	}
	q = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
	return &q
}
