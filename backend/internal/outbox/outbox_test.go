package outbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/textproto"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/mail"
	"github.com/MboyaDan/dakota-college/backend/internal/testdb"
)

// flaky fails the first `failures` sends with err, then records messages.
type flaky struct {
	mu       sync.Mutex
	failures int
	err      error
	sent     []mail.Message
	tries    int
}

func (f *flaky) Send(_ context.Context, m mail.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tries++
	if f.failures > 0 {
		f.failures--
		return f.err
	}
	f.sent = append(f.sent, m)
	return nil
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time      { return c.t }
func (c *clock) add(d time.Duration) { c.t = c.t.Add(d) }

type row struct {
	Status   string
	Attempts int
	Error    string
	Payload  []byte
	SentAt   *time.Time
}

func setup(t *testing.T, m mail.Mailer) (*Queue, *pgxpool.Pool, *clock) {
	t.Helper()
	pool, _ := testdb.Migrated(t)
	c := &clock{t: time.Now()}
	q, err := New(Options{Pool: pool, Mailer: m, Key: bytes.Repeat([]byte{7}, KeySize), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: c.now})
	if err != nil {
		t.Fatal(err)
	}
	return q, pool, c
}

func get(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) row {
	t.Helper()
	var r row
	err := pool.QueryRow(context.Background(), `SELECT status, attempts, error, payload, sent_at FROM notifications WHERE id = $1`, id).
		Scan(&r.Status, &r.Attempts, &r.Error, &r.Payload, &r.SentAt)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func process(t *testing.T, q *Queue) int {
	t.Helper()
	n, err := q.Process(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return n
}

var message = mail.Message{To: "test.student.001@example.invalid", Subject: "Test subject", Text: "Open https://dakota.example/x?token=secret"}

func TestSentMessageKeepsNoText(t *testing.T) {
	m := &flaky{}
	q, pool, _ := setup(t, m)
	id, err := q.Email(context.Background(), Item{SystemKey: SystemPasswordReset}, message)
	if err != nil {
		t.Fatal(err)
	}
	r := get(t, pool, id)
	if r.Status != "queued" || len(r.Payload) == 0 || bytes.Contains(r.Payload, []byte("token=secret")) {
		t.Fatalf("queued row must hold only encrypted text: %+v", r)
	}
	if n := process(t, q); n != 1 {
		t.Fatalf("tried %d", n)
	}
	r = get(t, pool, id)
	if r.Status != "sent" || r.Payload != nil || r.SentAt == nil || r.Attempts != 1 {
		t.Fatalf("after sending: %+v", r)
	}
	if len(m.sent) != 1 || m.sent[0] != message {
		t.Fatalf("sent %+v", m.sent)
	}
	if n := process(t, q); n != 0 || len(m.sent) != 1 {
		t.Fatal("a sent message went again")
	}
	// The database refuses text on a sent message.
	if _, err := pool.Exec(context.Background(), `UPDATE notifications SET payload = '\x00' WHERE id = $1`, id); err == nil {
		t.Fatal("a sent message may not keep text")
	}
}

func TestFailuresAreRetriedWithGrowingWaits(t *testing.T) {
	m := &flaky{failures: 3, err: errors.New("dial tcp: connection refused")}
	q, pool, c := setup(t, m)
	id, err := q.Email(context.Background(), Item{SystemKey: SystemEnquiryReceived}, message)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 3 {
		if n := process(t, q); n != 1 {
			t.Fatalf("attempt %d: tried %d", i+1, n)
		}
		r := get(t, pool, id)
		if r.Status != "queued" || r.Attempts != i+1 || r.Error != "dial tcp: connection refused" {
			t.Fatalf("after failure %d: %+v", i+1, r)
		}
		// Not before the wait is over.
		c.add(RetryDelays[i] - time.Second)
		if n := process(t, q); n != 0 {
			t.Fatalf("retried %v early", RetryDelays[i])
		}
		c.add(time.Second)
	}
	process(t, q)
	if r := get(t, pool, id); r.Status != "sent" || r.Attempts != 4 || r.Error != "" || r.Payload != nil {
		t.Fatalf("fourth attempt: %+v", r)
	}
}

func TestGivesUpAfterTheLastAttemptAndCanBeSentAgain(t *testing.T) {
	m := &flaky{failures: 100, err: errors.New("timeout")}
	q, pool, c := setup(t, m)
	id, _ := q.Email(context.Background(), Item{SystemKey: SystemEnquiryAssigned}, message)
	for _, d := range append(RetryDelays, 0) {
		process(t, q)
		c.add(d)
	}
	r := get(t, pool, id)
	if r.Status != "failed" || r.Attempts != len(RetryDelays)+1 || len(r.Payload) == 0 {
		t.Fatalf("after the last attempt: %+v", r)
	}
	c.add(24 * time.Hour)
	if n := process(t, q); n != 0 {
		t.Fatal("a failed message was tried again by itself")
	}
	// What notify.Resend does.
	if _, err := pool.Exec(context.Background(), `UPDATE notifications SET status = 'queued', attempts = 0, next_attempt_at = $2 WHERE id = $1`, id, c.now()); err != nil {
		t.Fatal(err)
	}
	m.failures = 0
	process(t, q)
	if r := get(t, pool, id); r.Status != "sent" || len(m.sent) != 1 {
		t.Fatalf("sent again: %+v", r)
	}
}

func TestRefusedAddressFailsAtOnce(t *testing.T) {
	for name, err := range map[string]error{
		"bad address":     fmt.Errorf("recipient: %w", mail.ErrBadAddress),
		"mailbox refused": &textproto.Error{Code: 550, Msg: "no such user"},
	} {
		t.Run(name, func(t *testing.T) {
			q, pool, _ := setup(t, &flaky{failures: 1, err: err})
			id, _ := q.Email(context.Background(), Item{SystemKey: SystemEnquiryAssigned}, message)
			process(t, q)
			if r := get(t, pool, id); r.Status != "failed" || r.Attempts != 1 {
				t.Fatalf("%+v", r)
			}
		})
	}
	if Permanent(&textproto.Error{Code: 535, Msg: "authentication failed"}) || Permanent(&textproto.Error{Code: 421}) {
		t.Fatal("our sign-in problems and busy servers must be retried")
	}
}

func TestExpiredMessagesAreNotSent(t *testing.T) {
	m := &flaky{failures: 1, err: errors.New("timeout")}
	q, pool, c := setup(t, m)
	id, _ := q.Email(context.Background(), Item{SystemKey: SystemPasswordReset, ValidFor: 90 * time.Second}, message)
	process(t, q) // fails; retry in a minute
	c.add(2 * time.Minute)
	if n := process(t, q); n != 0 || len(m.sent) != 0 {
		t.Fatal("a message was sent after its link expired")
	}
	if r := get(t, pool, id); r.Status != "failed" || r.Error != "Not sent before it expired" || r.Payload != nil {
		t.Fatalf("expired: %+v", r)
	}

	// A failed message's text is erased when it expires.
	m.failures = 1
	m.err = &textproto.Error{Code: 550}
	id, _ = q.Email(context.Background(), Item{SystemKey: SystemEnquiryAssigned, ValidFor: time.Hour}, message)
	process(t, q)
	if r := get(t, pool, id); r.Status != "failed" || len(r.Payload) == 0 {
		t.Fatalf("failed: %+v", r)
	}
	c.add(time.Hour)
	process(t, q)
	if r := get(t, pool, id); r.Payload != nil {
		t.Fatal("expired text kept")
	}
}

func TestAbandonedClaimIsTakenAgain(t *testing.T) {
	m := &flaky{}
	q, pool, c := setup(t, m)
	id, _ := q.Email(context.Background(), Item{SystemKey: SystemEnquiryReceived}, message)
	// A worker claims the message and stops before sending.
	if _, err := pool.Exec(context.Background(), `UPDATE notifications SET status = 'sending', attempts = 1, locked_until = $2 WHERE id = $1`, id, c.now().Add(claimFor)); err != nil {
		t.Fatal(err)
	}
	if n := process(t, q); n != 0 {
		t.Fatal("a claimed message was taken while its claim held")
	}
	c.add(claimFor + time.Second)
	process(t, q)
	if r := get(t, pool, id); r.Status != "sent" || r.Attempts != 2 {
		t.Fatalf("%+v", r)
	}
}

func TestChangedRecipientCannotReadTheMessage(t *testing.T) {
	m := &flaky{}
	q, pool, _ := setup(t, m)
	id, _ := q.Email(context.Background(), Item{SystemKey: SystemPasswordReset}, message)
	if _, err := pool.Exec(context.Background(), `UPDATE notifications SET recipient = 'someone.else@example.invalid' WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	process(t, q)
	r := get(t, pool, id)
	if r.Status != "failed" || len(m.sent) != 0 || r.Attempts != 1 {
		t.Fatalf("redirected message: %+v, sent %d", r, len(m.sent))
	}
}

func TestWorkersShareTheQueue(t *testing.T) {
	m := &flaky{}
	q, pool, c := setup(t, m)
	other, err := New(Options{Pool: pool, Mailer: m, Key: bytes.Repeat([]byte{7}, KeySize), Logger: q.logger, Now: c.now})
	if err != nil {
		t.Fatal(err)
	}
	const n = 35
	for i := range n {
		if _, err := q.Email(context.Background(), Item{SystemKey: SystemEnquiryReceived, RelatedID: fmt.Sprint(i)}, message); err != nil {
			t.Fatal(err)
		}
	}
	var wg sync.WaitGroup
	for _, w := range []*Queue{q, other, q, other} {
		wg.Go(func() {
			if _, err := w.Process(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if m.tries != n {
		t.Fatalf("%d sends for %d messages", m.tries, n)
	}
}

func TestTextMessagesAreSkippedWithoutAProvider(t *testing.T) {
	q, pool, _ := setup(t, &flaky{})
	id, err := q.SMS(context.Background(), Item{TemplateKey: "application.submitted"}, "+254700000000", "")
	if err != nil {
		t.Fatal(err)
	}
	if r := get(t, pool, id); r.Status != "skipped" || r.Payload != nil || r.Error != ErrSMSUnavailable.Error() {
		t.Fatalf("%+v", r)
	}
	if _, err := q.Email(context.Background(), Item{}, message); err == nil {
		t.Fatal("a message needs a template or system key")
	}
}
