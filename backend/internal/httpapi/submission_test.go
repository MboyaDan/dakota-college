package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/mail"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
)

var testPDF = []byte("%PDF-1.4\n1 0 obj << /Type /Catalog >> endobj\ntrailer << /Root 1 0 R >>\n%%EOF\n")

func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	for x := range 40 {
		img.Set(x, x%30, color.RGBA{200, 10, 10, 255})
	}
	var b bytes.Buffer
	if err := jpeg.Encode(&b, img, nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func (h *harness) uploadDoc(s *session, appID, kind, filename string, data []byte) *httpResponse {
	h.t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("kind", kind)
	fw, _ := w.CreateFormFile("file", filename)
	_, _ = fw.Write(data)
	_ = w.Close()
	rec := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/applications/" + appID + "/documents", raw: &body, contentType: w.FormDataContentType(), sess: s})
	return &httpResponse{rec.Code, rec.Body.String()}
}

// readyApplication is an adult's application with every step filled in;
// only documents (and the declaration) are left.
func (h *harness) readyApplication(email string) (*applicantSession, string) {
	h.t.Helper()
	slug, _, _ := h.openProgramme(1)
	a := h.newApplicant(email, "Test Applicant")
	id := idOf(h.t, h.start(a.session, slug, nil))
	h.put(a.session, id, "personal", map[string]any{
		"first_name": "Test", "last_name": "Applicant 020", "date_of_birth": time.Now().AddDate(-20, 0, 0).Format(time.DateOnly),
		"identity_document_type": "national_id", "identity_document": "00000020", "nationality": "Kenyan", "county": "Kisumu",
		"phone": "+254 700 000 020",
	}, http.StatusOK)
	h.put(a.session, id, "education", map[string]any{"items": []map[string]any{{"level": "kcse", "institution": "Test School", "mean_grade": "B"}}}, http.StatusOK)
	h.put(a.session, id, "sponsor", map[string]any{"type": "self"}, http.StatusOK)
	return a, id
}

func uploadedID(t *testing.T, r *httpResponse) string {
	t.Helper()
	var d struct{ ID string }
	if err := json.Unmarshal([]byte(r.body), &d); err != nil || d.ID == "" {
		t.Fatalf("no document id: %s", r.body)
	}
	return d.ID
}

func (h *harness) uploadAll(s *session, id string) {
	h.t.Helper()
	h.uploadDoc(s, id, "national_id", "id.pdf", testPDF).expect(h.t, http.StatusCreated, "")
	h.uploadDoc(s, id, "passport_photo", "photo.jpg", testJPEG(h.t)).expect(h.t, http.StatusCreated, "")
	h.uploadDoc(s, id, "kcse_certificate", "kcse.pdf", testPDF).expect(h.t, http.StatusCreated, "")
}

func (h *harness) privateObjects() int {
	n := 0
	for _, k := range h.storage.Keys() {
		if strings.HasPrefix(k, "dakota-private/") {
			n++
		}
	}
	return n
}

// AC-05: documents are checked by content and kept privately.
func TestDocumentUploads(t *testing.T) {
	h := newHarness(t)
	a, id := h.readyApplication("applicant020@example.invalid")

	d := h.app(req{method: http.MethodGet, path: "/api/v1/applicant/applications/" + id, sess: a.session}, http.StatusOK)
	if got := d.missingSteps(); !slices.Equal(got, []string{"documents"}) {
		t.Fatalf("missing = %v", got)
	}

	pdf := h.uploadDoc(a.session, id, "national_id", "My ID card.pdf", testPDF)
	pdf.expect(t, http.StatusCreated, "")
	if !strings.Contains(pdf.body, `"scan_status":"not_scanned"`) || !strings.Contains(pdf.body, `"name":"My ID card.pdf"`) {
		t.Fatalf("upload: %s", pdf.body)
	}
	// A photo is stored re-encoded, as JPEG.
	photo := h.uploadDoc(a.session, id, "passport_photo", "photo.png", testJPEG(t))
	photo.expect(t, http.StatusCreated, "")
	if !strings.Contains(photo.body, `"name":"photo.jpg"`) || !strings.Contains(photo.body, `"content_type":"image/jpeg"`) {
		t.Fatalf("photo: %s", photo.body)
	}

	// Checked by content, not name.
	h.uploadDoc(a.session, id, "kcse_certificate", "kcse.pdf", []byte("<html><script>alert(1)</script></html>")).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	h.uploadDoc(a.session, id, "kcse_certificate", "kcse.pdf", nil).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	h.uploadDoc(a.session, id, "library_card", "x.pdf", testPDF).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	big := append(append([]byte{}, testPDF...), bytes.Repeat([]byte("x"), media.MaxUploadBytes)...)
	h.uploadDoc(a.session, id, "kcse_certificate", "big.pdf", big).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	if n := h.privateObjects(); n != 2 {
		t.Fatalf("private objects = %d, want 2", n)
	}

	// Nobody else can add to it.
	other := h.newApplicant("applicant021@example.invalid", "")
	h.uploadDoc(other.session, id, "other", "x.pdf", testPDF).expect(t, http.StatusNotFound, "not_found")
	if n := h.privateObjects(); n != 2 {
		t.Fatalf("a refused upload was stored (%d objects)", n)
	}

	kcse := h.uploadDoc(a.session, id, "kcse_certificate", "kcse.pdf", testPDF)
	kcse.expect(t, http.StatusCreated, "")
	d = h.app(req{method: http.MethodGet, path: "/api/v1/applicant/applications/" + id, sess: a.session}, http.StatusOK)
	if len(d.Missing) != 0 {
		t.Fatalf("still missing: %+v", d.Missing)
	}

	// Removing one takes it out of storage and back onto the missing list.
	d = h.app(req{method: http.MethodDelete, path: "/api/v1/applicant/applications/" + id + "/documents/" + uploadedID(t, kcse), sess: a.session}, http.StatusOK)
	if !slices.Contains(d.missingSteps(), "documents") || h.privateObjects() != 2 {
		t.Fatalf("after remove: missing %v, objects %d", d.missingSteps(), h.privateObjects())
	}
	var deleted int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM files WHERE deleted_at IS NOT NULL`).Scan(&deleted); err != nil || deleted != 1 {
		t.Fatalf("deleted file rows = %d (%v)", deleted, err)
	}

	// At most 12.
	for range 10 {
		h.uploadDoc(a.session, id, "other", "x.pdf", testPDF).expect(t, http.StatusCreated, "")
	}
	h.uploadDoc(a.session, id, "other", "x.pdf", testPDF).expect(t, http.StatusUnprocessableEntity, "validation_failed")
}

type fakeScanner struct {
	status string
	err    error
}

func (f fakeScanner) Scan(context.Context, []byte) (string, error) { return f.status, f.err }

func TestDocumentScanning(t *testing.T) {
	h := newHarness(t)
	a, id := h.readyApplication("applicant022@example.invalid")

	h.scanner = fakeScanner{status: media.ScanInfected}
	h.rebuild()
	h.uploadDoc(a.session, id, "national_id", "id.pdf", testPDF).expect(t, http.StatusUnprocessableEntity, "file_rejected")
	h.scanner = fakeScanner{err: errors.New("clamd down")}
	h.rebuild()
	h.uploadDoc(a.session, id, "national_id", "id.pdf", testPDF).expect(t, http.StatusServiceUnavailable, "scan_unavailable")
	if n := h.privateObjects(); n != 0 {
		t.Fatalf("refused files were stored: %d", n)
	}
	h.scanner = fakeScanner{status: media.ScanClean}
	h.rebuild()
	r := h.uploadDoc(a.session, id, "national_id", "id.pdf", testPDF)
	r.expect(t, http.StatusCreated, "")
	if !strings.Contains(r.body, `"scan_status":"clean"`) {
		t.Fatalf("clean upload: %s", r.body)
	}
}

func (h *harness) link(r req) string {
	h.t.Helper()
	w := h.do(r)
	if w.Code != http.StatusOK {
		h.t.Fatalf("link %s: %d %s", r.path, w.Code, w.Body.String())
	}
	var l struct{ URL string }
	if err := json.Unmarshal(w.Body.Bytes(), &l); err != nil || !strings.HasPrefix(l.URL, "/api/v1/files/") {
		h.t.Fatalf("link body: %s", w.Body.String())
	}
	return l.URL
}

// AC-05: private documents are reachable only through short-lived links,
// by the account the link was issued to, and staff only once submitted.
func TestDocumentLinks(t *testing.T) {
	h := newHarness(t)
	a, id := h.readyApplication("applicant023@example.invalid")
	docID := uploadedID(t, h.uploadDoc(a.session, id, "national_id", "id.pdf", testPDF))
	h.uploadDoc(a.session, id, "passport_photo", "photo.jpg", testJPEG(t)).expect(t, http.StatusCreated, "")
	h.uploadDoc(a.session, id, "kcse_certificate", "kcse.pdf", testPDF).expect(t, http.StatusCreated, "")
	docPath := "/api/v1/applicant/applications/" + id + "/documents/" + docID

	url := h.link(req{method: http.MethodPost, path: docPath + "/link", sess: a.session})
	w := h.do(req{method: http.MethodGet, path: url, sess: a.session})
	if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), testPDF) {
		t.Fatalf("own link: %d", w.Code)
	}
	for k, want := range map[string]string{
		"Content-Type": "application/pdf", "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff",
		"Content-Disposition": `attachment; filename=id.pdf`,
	} {
		if got := w.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if !strings.Contains(w.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Error("no sandbox CSP on a private file")
	}

	// Not for anyone else, not without a session, not after five minutes.
	other := h.newApplicant("applicant024@example.invalid", "")
	expectStatus(t, h.do(req{method: http.MethodGet, path: url, sess: other.session}), http.StatusNotFound, "link_expired")
	expectStatus(t, h.do(req{method: http.MethodGet, path: url}), http.StatusUnauthorized, "unauthenticated")
	expectStatus(t, h.do(req{method: http.MethodPost, path: docPath + "/link", sess: other.session}), http.StatusNotFound, "not_found")
	h.now = func() time.Time { return time.Now().Add(6 * time.Minute) }
	expectStatus(t, h.do(req{method: http.MethodGet, path: url, sess: a.session}), http.StatusNotFound, "link_expired")
	h.now = time.Now
	// Only a hash of the token is kept.
	token := strings.TrimPrefix(url, "/api/v1/files/")
	var plain int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM file_links WHERE encode(token_hash, 'escape') LIKE '%' || $1 || '%'`, token).Scan(&plain); err != nil || plain != 0 {
		t.Fatalf("token stored in plain (%v)", err)
	}

	// Staff: admissions.view, and only after submission.
	h.createUser("admissions@example.invalid", "admissions_officer")
	h.createUser("editor@example.invalid", "content_editor")
	h.createUser("admissions2@example.invalid", "admissions_officer")
	officer := h.mustLogin("admissions@example.invalid")
	staffPath := "/api/v1/staff/applications/" + id + "/documents/" + docID + "/link"
	expectStatus(t, h.do(req{method: http.MethodPost, path: staffPath, sess: officer}), http.StatusNotFound, "not_found")
	h.submit(a.session, id, "key-links-0001", true).expect(t, http.StatusOK, "")
	staffURL := h.link(req{method: http.MethodPost, path: staffPath, sess: officer})
	if w := h.do(req{method: http.MethodGet, path: staffURL, sess: officer}); w.Code != http.StatusOK {
		t.Fatalf("officer link: %d", w.Code)
	}
	expectStatus(t, h.do(req{method: http.MethodGet, path: staffURL, sess: h.mustLogin("admissions2@example.invalid")}), http.StatusNotFound, "link_expired")
	expectStatus(t, h.do(req{method: http.MethodPost, path: staffPath, sess: h.mustLogin("editor@example.invalid")}), http.StatusForbidden, "forbidden")
	if n := h.auditCount("file.link_issued"); n != 1 {
		t.Fatalf("staff links audited %d times, want 1", n)
	}
}

func (h *harness) submit(s *session, id, key string, declaration bool) *httpResponse {
	h.t.Helper()
	w := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/applications/" + id + "/submit", sess: s,
		body: map[string]any{"declaration": declaration}, headers: map[string]string{"Idempotency-Key": key}})
	return &httpResponse{w.Code, w.Body.String()}
}

// AC-04: submitted once; a retry never creates a duplicate.
func TestSubmitOnce(t *testing.T) {
	h := newHarness(t)
	a, id := h.readyApplication("applicant025@example.invalid")

	r := h.submit(a.session, id, "key-submit-0001", true)
	r.expect(t, http.StatusUnprocessableEntity, "validation_failed")
	if !strings.Contains(r.body, `"field":"documents.national_id"`) {
		t.Fatalf("missing documents not listed: %s", r.body)
	}
	h.uploadAll(a.session, id)
	h.submit(a.session, id, "key-submit-0001", false).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	h.submit(a.session, id, "short", true).expect(t, http.StatusUnprocessableEntity, "validation_failed")
	// Failed attempts leave no record, so the same key works once fixed.

	// Five copies of the same request at once (a double tap on a poor line).
	type result struct {
		code     int
		body     string
		replayed bool
	}
	results := make([]result, 5)
	var wg sync.WaitGroup
	for i := range results {
		wg.Go(func() {
			w := h.do(req{method: http.MethodPost, path: "/api/v1/applicant/applications/" + id + "/submit", sess: a.session,
				body: map[string]any{"declaration": true}, headers: map[string]string{"Idempotency-Key": "key-submit-0001"}})
			results[i] = result{w.Code, w.Body.String(), w.Header().Get("Idempotent-Replayed") == "true"}
		})
	}
	wg.Wait()
	replayed := 0
	for _, res := range results {
		if res.code != http.StatusOK || res.body != results[0].body || !strings.Contains(res.body, `"status":"submitted"`) {
			t.Fatalf("results differ: %+v", results)
		}
		if res.replayed {
			replayed++
		}
	}
	if replayed != 4 {
		t.Fatalf("replayed = %d, want 4", replayed)
	}

	ctx := context.Background()
	var history, audits, consents, apps int
	if err := h.pool.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM application_status_history WHERE to_status = 'submitted'),
		(SELECT count(*) FROM audit_logs WHERE action = 'application.submitted'),
		(SELECT count(*) FROM consents WHERE purpose = 'data_processing'),
		(SELECT count(*) FROM applications)`).Scan(&history, &audits, &consents, &apps); err != nil {
		t.Fatal(err)
	}
	if history != 1 || audits != 1 || consents != 1 || apps != 1 {
		t.Fatalf("history %d, audits %d, consents %d, applications %d: want 1 each", history, audits, consents, apps)
	}

	// Another key: refused, nothing changes. Same key, other request: refused.
	h.submit(a.session, id, "key-submit-0002", true).expect(t, http.StatusConflict, "already_submitted")
	h.submit(a.session, id, "key-submit-0001", false).expect(t, http.StatusUnprocessableEntity, "idempotency_key_reused")

	// Read-only from now on, with its history and a frozen copy.
	d := h.app(req{method: http.MethodGet, path: "/api/v1/applicant/applications/" + id, sess: a.session}, http.StatusOK)
	if d.Status != "submitted" || d.Editable {
		t.Fatalf("after submit: %+v", d)
	}
	h.put(a.session, id, "sponsor", map[string]any{"type": "self"}, http.StatusConflict)
	h.uploadDoc(a.session, id, "other", "x.pdf", testPDF).expect(t, http.StatusConflict, "not_editable")
	var snapshot string
	if err := h.pool.QueryRow(ctx, `SELECT submitted_snapshot::text FROM applications WHERE id = $1`, id).Scan(&snapshot); err != nil ||
		!strings.Contains(snapshot, `"reference"`) || !strings.Contains(snapshot, `"documents"`) || strings.Contains(snapshot, `"missing"`) || strings.Contains(snapshot, `"status"`) {
		t.Fatalf("snapshot = %s (%v)", snapshot, err)
	}

	// One confirmation email, and the SMS recorded as skipped (DG-03).
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && !slices.ContainsFunc(h.mailer.All(), func(m mail.Message) bool { return strings.HasPrefix(m.Subject, "We received your application") }) {
		time.Sleep(10 * time.Millisecond)
	}
	n := 0
	for _, m := range h.mailer.All() {
		if strings.HasPrefix(m.Subject, "We received your application") {
			n++
		}
	}
	var sms string
	if err := h.pool.QueryRow(ctx, `SELECT status FROM notifications WHERE channel = 'sms' AND template_key = 'application.submitted'`).Scan(&sms); err != nil || n != 1 || sms != "skipped" {
		t.Fatalf("emails %d, sms %q (%v)", n, sms, err)
	}
}

// Many submits with different keys: exactly one succeeds.
func TestConcurrentSubmitsWithDifferentKeys(t *testing.T) {
	h := newHarness(t)
	a, id := h.readyApplication("applicant026@example.invalid")
	h.uploadAll(a.session, id)
	codes := make(chan int, 4)
	for range 4 {
		go func() { codes <- h.submit(a.session, id, "key-"+uuid.NewString(), true).code }()
	}
	got := []int{<-codes, <-codes, <-codes, <-codes}
	slices.Sort(got)
	if !slices.Equal(got, []int{http.StatusOK, http.StatusConflict, http.StatusConflict, http.StatusConflict}) {
		t.Fatalf("codes = %v", got)
	}
}
