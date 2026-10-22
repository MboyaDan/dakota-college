package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MboyaDan/dakota-college/backend/internal/mail"
)

func (h *harness) site() map[string]any {
	h.t.Helper()
	return h.call(req{method: http.MethodGet, path: "/api/v1/public/site"}, http.StatusOK)
}

func slugsOf(list any) []string {
	var out []string
	for _, item := range list.([]any) {
		out = append(out, item.(map[string]any)["slug"].(string))
	}
	return out
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// The public snapshot shows published content only, hides programmes of an
// unpublished school, upcoming intakes only, and no internal settings.
func TestPublicSiteShowsOnlyPublishedContent(t *testing.T) {
	h := newHarness(t)
	h.createUser("registrar@example.invalid", "content_editor", "registrar")
	reg := h.mustLogin("registrar@example.invalid")

	schoolID := h.create(reg, "schools", schoolDoc("school-of-testing"))
	progID := h.create(reg, "programmes", programmeDoc(schoolID, "diploma-in-testing"))
	draftDoc := programmeDoc(schoolID, "draft-programme")
	draftDoc["intakes"] = []any{}
	h.create(reg, "programmes", draftDoc)

	site := h.site()
	if len(site["schools"].([]any)) != 0 || len(site["programmes"].([]any)) != 0 {
		t.Fatalf("drafts are public: %v", site)
	}

	h.publish(reg, "schools", schoolID, http.StatusOK)
	h.publish(reg, "programmes", progID, http.StatusOK)
	// A past intake must not be offered.
	h.exec(`INSERT INTO programme_intakes (programme_id, name, starts_on, status) VALUES ($1, 'Past', DATE '2020-01-06', 'open')`, progID)

	site = h.site()
	if got := slugsOf(site["schools"]); len(got) != 1 || got[0] != "school-of-testing" {
		t.Fatalf("schools %v", got)
	}
	progs := slugsOf(site["programmes"])
	if len(progs) != 1 || progs[0] != "diploma-in-testing" {
		t.Fatalf("programmes %v, want only the published one", progs)
	}
	p := site["programmes"].([]any)[0].(map[string]any)
	if p["school_slug"] != "school-of-testing" || len(p["requirements"].([]any)) != 1 || len(p["fees"].([]any)) != 1 {
		t.Fatalf("programme detail incomplete: %v", p)
	}
	if fee := p["fees"].([]any)[0].(map[string]any); fee["amount_cents"] != float64(4500000) {
		t.Fatalf("fee %v", fee)
	}
	intakes := p["intakes"].([]any)
	if len(intakes) != 1 || intakes[0].(map[string]any)["starts_on"] != "2027-01-11" {
		t.Fatalf("intakes %v, want only the upcoming one", intakes)
	}

	settings := site["settings"].(map[string]any)
	if _, ok := settings["contact.phone"]; !ok {
		t.Fatal("public settings missing")
	}
	for k := range settings {
		if strings.HasPrefix(k, "student_number.") || strings.HasPrefix(k, "applications.") {
			t.Fatalf("internal setting %s is public", k)
		}
	}

	// Unpublishing the school hides its programmes too.
	h.call(req{method: http.MethodPost, path: contentPath("schools", schoolID, "unpublish"), body: map[string]any{}, sess: reg}, http.StatusOK)
	if got := h.site()["programmes"].([]any); len(got) != 0 {
		t.Fatalf("programme of an unpublished school is public: %v", got)
	}
}

// AC-16: unverified figures and accreditations never appear.
func TestPublicSiteHidesUnverifiedClaims(t *testing.T) {
	h := newHarness(t)
	h.exec(`INSERT INTO statistics (label, value, status) VALUES ('Graduates', '1000+', 'draft')`)
	h.exec(`INSERT INTO statistics (label, value, source_note, verified_on, status) VALUES ('Trainers', '12', 'Staff list', DATE '2026-10-01', 'published')`)
	h.exec(`INSERT INTO accreditations (body, reference, status) VALUES ('TVETA', 'X/1', 'draft')`)

	site := h.site()
	stats := site["statistics"].([]any)
	if len(stats) != 1 || stats[0].(map[string]any)["label"] != "Trainers" || stats[0].(map[string]any)["verified_on"] != "2026-10-01" {
		t.Fatalf("statistics %v", stats)
	}
	if len(site["accreditations"].([]any)) != 0 {
		t.Fatalf("unverified accreditation is public")
	}
}

func enquiryBody(overrides map[string]any) map[string]any {
	b := map[string]any{
		"full_name": "Test Visitor", "email": "visitor@example.invalid", "phone": "", "query_type": "admissions",
		"programme": "", "message": "When does the next intake start?", "website": "", "elapsed_ms": 15000,
	}
	for k, v := range overrides {
		b[k] = v
	}
	return b
}

func (h *harness) sendEnquiry(body map[string]any, wantStatus int) map[string]any {
	h.t.Helper()
	return h.call(req{method: http.MethodPost, path: "/api/v1/public/enquiries", body: body, headers: map[string]string{"Origin": testOrigin}}, wantStatus)
}

func (h *harness) enquiryCount() int {
	h.t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM enquiries`).Scan(&n); err != nil {
		h.t.Fatal(err)
	}
	return n
}

// waitForMail waits for the background send.
func (h *harness) waitForMail(n int) []mail.Message {
	h.t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if msgs := h.mailer.All(); len(msgs) >= n {
			return msgs
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatalf("expected %d emails, got %d", n, len(h.mailer.All()))
	return nil
}

func (h *harness) setSetting(key, value string) {
	h.exec(`UPDATE site_settings SET value = $2 WHERE key = $1`, key, value)
}

// Stage 6: the contact form saves the enquiry and emails the right office.
func TestEnquiryIsSavedAndRoutedToTheRightOffice(t *testing.T) {
	h := newHarness(t)
	h.setSetting("contact.email", "info@example.invalid")
	h.setSetting("contact.admissions_email", "admissions@example.invalid")
	h.setSetting("contact.finance_email", "")

	h.sendEnquiry(enquiryBody(nil), http.StatusAccepted)
	msgs := h.waitForMail(1)
	m := msgs[0]
	if m.To != "admissions@example.invalid" || m.ReplyTo != "visitor@example.invalid" || !strings.Contains(m.Text, "When does the next intake start?") {
		t.Fatalf("admissions email wrong: %+v", m)
	}
	if strings.ContainsAny(m.Subject, "\r\n") || strings.Contains(m.Subject, "Test Visitor") {
		t.Fatalf("subject must not carry visitor text: %q", m.Subject)
	}

	// Fees go to finance; with no finance address, to the general address.
	h.sendEnquiry(enquiryBody(map[string]any{"query_type": "fees", "email": "", "phone": "+254 000 000 001"}), http.StatusAccepted)
	m = h.waitForMail(2)[1]
	if m.To != "info@example.invalid" || m.ReplyTo != "" || !strings.Contains(m.Text, "+254 000 000 001") {
		t.Fatalf("fees fallback email wrong: %+v", m)
	}

	var queryType, source string
	if err := h.pool.QueryRow(context.Background(), `SELECT query_type, source FROM enquiries WHERE phone = '+254 000 000 001'`).Scan(&queryType, &source); err != nil {
		t.Fatal(err)
	}
	if queryType != "fees" || source != "website" {
		t.Fatalf("saved %s/%s", queryType, source)
	}

	// No office address at all: still saved, not emailed.
	h.setSetting("contact.email", "")
	h.sendEnquiry(enquiryBody(map[string]any{"query_type": "other", "email": "other@example.invalid"}), http.StatusAccepted)
	if h.enquiryCount() != 3 {
		t.Fatalf("enquiries saved: %d, want 3", h.enquiryCount())
	}
	time.Sleep(50 * time.Millisecond)
	if n := len(h.mailer.All()); n != 2 {
		t.Fatalf("emails %d, want 2", n)
	}
}

func TestEnquiryLinksToPublishedProgrammeOnly(t *testing.T) {
	h := newHarness(t)
	h.createUser("registrar@example.invalid", "content_editor", "registrar")
	reg := h.mustLogin("registrar@example.invalid")
	schoolID := h.create(reg, "schools", schoolDoc("school-of-testing"))
	progID := h.create(reg, "programmes", programmeDoc(schoolID, "diploma-in-testing"))

	w := h.do(req{method: http.MethodPost, path: "/api/v1/public/enquiries", body: enquiryBody(map[string]any{"programme": "diploma-in-testing"})})
	expectStatus(t, w, http.StatusUnprocessableEntity, "validation_failed")

	h.publish(reg, "schools", schoolID, http.StatusOK)
	h.publish(reg, "programmes", progID, http.StatusOK)
	h.sendEnquiry(enquiryBody(map[string]any{"programme": "diploma-in-testing"}), http.StatusAccepted)
	var linked string
	if err := h.pool.QueryRow(context.Background(), `SELECT programme_id::text FROM enquiries`).Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if linked != progID {
		t.Fatalf("programme %s, want %s", linked, progID)
	}
}

func TestEnquiryValidation(t *testing.T) {
	h := newHarness(t)
	cases := map[string]struct {
		body  map[string]any
		field string
	}{
		"no contact details": {enquiryBody(map[string]any{"email": "", "phone": ""}), "email"},
		"bad email":          {enquiryBody(map[string]any{"email": "not-an-email"}), "email"},
		"bad phone":          {enquiryBody(map[string]any{"email": "", "phone": "call me"}), "phone"},
		"no name":            {enquiryBody(map[string]any{"full_name": "  "}), "full_name"},
		"unknown query type": {enquiryBody(map[string]any{"query_type": "jobs"}), "query_type"},
		"empty message":      {enquiryBody(map[string]any{"message": ""}), "message"},
		"too many links":     {enquiryBody(map[string]any{"message": "see https://a.example http://b.example www.c.example"}), "message"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			w := h.do(req{method: http.MethodPost, path: "/api/v1/public/enquiries", body: tc.body})
			expectStatus(t, w, http.StatusUnprocessableEntity, "validation_failed")
			if !strings.Contains(w.Body.String(), `"field":"`+tc.field+`"`) {
				t.Fatalf("no problem for %s: %s", tc.field, w.Body.String())
			}
		})
	}
	if h.enquiryCount() != 0 {
		t.Fatal("invalid enquiries were saved")
	}
}

// Spam protection: honeypot and too-fast submissions look accepted but are
// neither saved nor emailed; a fourth message in an hour from one address is
// refused.
func TestEnquirySpamProtection(t *testing.T) {
	h := newHarness(t)
	h.setSetting("contact.admissions_email", "admissions@example.invalid")

	h.sendEnquiry(enquiryBody(map[string]any{"website": "https://spam.example"}), http.StatusAccepted)
	h.sendEnquiry(enquiryBody(map[string]any{"elapsed_ms": 800}), http.StatusAccepted)
	if h.enquiryCount() != 0 {
		t.Fatal("spam was saved")
	}

	for i := 0; i < 3; i++ {
		h.sendEnquiry(enquiryBody(map[string]any{"email": "Repeat@example.invalid"}), http.StatusAccepted)
	}
	w := h.do(req{method: http.MethodPost, path: "/api/v1/public/enquiries", body: enquiryBody(map[string]any{"email": "repeat@example.invalid"})})
	expectStatus(t, w, http.StatusTooManyRequests, "too_many_enquiries")
	if h.enquiryCount() != 3 {
		t.Fatalf("enquiries %d, want 3", h.enquiryCount())
	}
	h.waitForMail(3)
	time.Sleep(50 * time.Millisecond)
	if n := len(h.mailer.All()); n != 3 {
		t.Fatalf("emails %d, want 3", n)
	}

	// Another site cannot post the form from a visitor's browser.
	w = h.do(req{method: http.MethodPost, path: "/api/v1/public/enquiries", body: enquiryBody(nil), headers: map[string]string{"Origin": "https://evil.example"}})
	expectStatus(t, w, http.StatusForbidden, "cross_site_request")
}
