package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"github.com/MboyaDan/dakota-college/backend/internal/notify"
)

// waitStatus waits for the queue to give the newest message to `to` the
// status.
func (h *harness) waitStatus(to, status string) string {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	var id, got string
	for time.Now().Before(deadline) {
		err := h.pool.QueryRow(context.Background(),
			`SELECT id::text, status FROM notifications WHERE recipient = $1 ORDER BY created_at DESC LIMIT 1`, to).Scan(&id, &got)
		if err == nil && got == status {
			return id
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("message to %s is %q, want %q", to, got, status)
	return ""
}

func (h *harness) sentLog(s *session, query string) notify.SentPage {
	h.t.Helper()
	w := h.do(req{method: http.MethodGet, path: "/api/v1/staff/notifications" + query, sess: s})
	expectStatus(h.t, w, http.StatusOK, "")
	var page notify.SentPage
	if err := json.Unmarshal(w.Body.Bytes(), &page); err != nil {
		h.t.Fatal(err)
	}
	return page
}

// Stage 16a: staff emails go through the queue; failures show in the log and
// can be sent again; the text is never shown.
func TestSentMessagesLogAndResend(t *testing.T) {
	h := newHarness(t)
	h.createUser("registrar@example.invalid", "registrar")
	h.createUser("editor@example.invalid", "content_editor")
	h.createUser("trainer@example.invalid", "trainer")
	registrar := h.mustLogin("registrar@example.invalid")
	editor := h.mustLogin("editor@example.invalid")

	// The receiving server refuses the mailbox: no retries.
	h.mailer.FailWith(&textproto.Error{Code: 550, Msg: "mailbox unavailable"})
	h.do(req{method: http.MethodPost, path: "/api/v1/auth/password-reset", body: map[string]string{"email": "trainer@example.invalid"}})
	id := h.waitStatus("trainer@example.invalid", "failed")

	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/notifications", sess: editor}), http.StatusForbidden, "forbidden")
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/staff/notifications/" + id + "/resend", sess: editor}), http.StatusForbidden, "forbidden")
	expectStatus(t, h.do(req{method: http.MethodGet, path: "/api/v1/staff/notifications?status=lost", sess: registrar}), http.StatusBadRequest, "invalid_request")

	w := h.do(req{method: http.MethodGet, path: "/api/v1/staff/notifications?status=failed&q=TRAINER@", sess: registrar})
	expectStatus(t, w, http.StatusOK, "")
	if strings.Contains(w.Body.String(), "reset-password") || strings.Contains(w.Body.String(), "token") {
		t.Fatalf("the log shows message text: %s", w.Body)
	}
	page := h.sentLog(registrar, "?status=failed&q=TRAINER@")
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("failed messages: %+v", page)
	}
	m := page.Items[0]
	if m.ID.String() != id || m.Name != "Password reset (staff and students)" || m.Channel != "email" || m.Attempts != 1 ||
		!strings.Contains(m.Error, "550") || !m.CanResend || m.RelatedType != "user" {
		t.Fatalf("log row: %+v", m)
	}
	w = h.do(req{method: http.MethodGet, path: "/api/v1/staff/notifications/counts", sess: registrar})
	if !strings.Contains(w.Body.String(), `"failed_recent":1`) {
		t.Fatalf("counts: %s", w.Body)
	}
	// Wildcards in the search box are taken literally.
	if p := h.sentLog(registrar, "?q=%25"); p.Total != 0 {
		t.Fatalf("%% matched %d messages", p.Total)
	}

	h.mailer.FailWith(nil)
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/staff/notifications/" + id + "/resend", sess: registrar}), http.StatusNoContent, "")
	h.waitForResetToken("trainer@example.invalid", 1)
	h.waitStatus("trainer@example.invalid", "sent")
	if n := h.auditCount("notifications.resent"); n != 1 {
		t.Fatalf("%d resend audit rows", n)
	}
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/staff/notifications/" + id + "/resend", sess: registrar}), http.StatusConflict, "not_resendable")
	var payload []byte
	if err := h.pool.QueryRow(context.Background(), `SELECT payload FROM notifications WHERE id = $1`, id).Scan(&payload); err != nil || payload != nil {
		t.Fatalf("a sent message kept its text: %v", err)
	}

	// A reset link that expired is not sent again.
	h.mailer.FailWith(&textproto.Error{Code: 550})
	h.do(req{method: http.MethodPost, path: "/api/v1/auth/password-reset", body: map[string]string{"email": "registrar@example.invalid"}})
	old := h.waitStatus("registrar@example.invalid", "failed")
	h.exec(`UPDATE notifications SET expires_at = now() - interval '1 minute' WHERE id = $1`, old)
	h.mailer.FailWith(nil)
	if p := h.sentLog(registrar, "?q=registrar@"); len(p.Items) != 1 || p.Items[0].CanResend {
		t.Fatalf("expired message offered for resend: %+v", p.Items)
	}
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/staff/notifications/" + old + "/resend", sess: registrar}), http.StatusConflict, "not_resendable")
	expectStatus(t, h.do(req{method: http.MethodPost, path: "/api/v1/staff/notifications/not-a-uuid/resend", sess: registrar}), http.StatusNotFound, "not_found")
}

// A mail server outage delays an applicant's email instead of losing it.
func TestApplicantEmailWaitsOutAnOutage(t *testing.T) {
	h := newHarness(t)
	h.mailer.FailWith(&textproto.Error{Code: 421, Msg: "service not available"})
	h.do(req{method: http.MethodPost, path: "/api/v1/applicant/sign-in-link", body: map[string]string{"email": "test.applicant.099@example.invalid"}})
	id := h.waitStatus("test.applicant.099@example.invalid", "queued")
	var attempts int
	var validFor time.Duration
	if err := h.pool.QueryRow(context.Background(),
		`SELECT attempts, (extract(epoch FROM expires_at - created_at) * 1e9)::bigint FROM notifications WHERE id = $1`, id).Scan(&attempts, &validFor); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || validFor > time.Hour {
		t.Fatalf("attempts %d, valid for %v (a sign-in link must expire with the link)", attempts, validFor)
	}
	h.mailer.FailWith(nil)
	// What the worker does when the retry falls due.
	h.exec(`UPDATE notifications SET next_attempt_at = now() WHERE id = $1`, id)
	h.queue.Wake()
	h.waitStatus("test.applicant.099@example.invalid", "sent")
}
