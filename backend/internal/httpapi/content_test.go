package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// call does a request and decodes the JSON answer into a map.
func (h *harness) call(r req, wantStatus int) map[string]any {
	h.t.Helper()
	w := h.do(r)
	if w.Code != wantStatus {
		h.t.Fatalf("%s %s: status %d, want %d: %s", r.method, r.path, w.Code, wantStatus, w.Body.String())
	}
	out := map[string]any{}
	if w.Body.Len() > 0 {
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			h.t.Fatalf("not JSON: %s", w.Body.String())
		}
	}
	return out
}

func contentPath(kind string, rest ...string) string {
	return "/api/v1/staff/content/" + kind + strings.Join(append([]string{""}, rest...), "/")
}

func (h *harness) create(s *session, kind string, doc map[string]any) string {
	h.t.Helper()
	return h.call(req{method: http.MethodPost, path: contentPath(kind), body: map[string]any{"document": doc}, sess: s}, http.StatusCreated)["id"].(string)
}

func (h *harness) item(s *session, kind, id string) map[string]any {
	h.t.Helper()
	return h.call(req{method: http.MethodGet, path: contentPath(kind, id), sess: s}, http.StatusOK)
}

func schoolDoc(slug string) map[string]any {
	return map[string]any{"slug": slug, "name": "School of Testing", "short_name": "Testing", "accent_key": "ict",
		"summary": "A school used by tests.", "description": "", "head_message": "", "sort_order": 1}
}

func programmeDoc(schoolID, slug string) map[string]any {
	return map[string]any{
		"school_id": schoolID, "slug": slug, "title": "Diploma in Testing", "level": "diploma", "mode": "full_time",
		"duration_months": 24, "duration_note": "", "min_kcse_grade": "C-", "summary": "Learn to test.", "overview": "",
		"what_you_learn": "Testing.", "career_outcomes": "", "assessment_model": "marks", "seo_title": "", "seo_description": "",
		"sort_order":   0,
		"requirements": []any{map[string]any{"kind": "academic", "requirement": "KCSE C- or equivalent."}},
		"intakes":      []any{map[string]any{"id": nil, "name": "January 2027", "starts_on": "2027-01-11", "applications_open_on": nil, "application_deadline": "2026-12-15", "capacity": nil, "status": "open"}},
		"fees":         []any{map[string]any{"label": "Tuition", "category": "tuition", "amount_cents": 4500000, "frequency": "per_term", "notes": ""}},
	}
}

func (h *harness) publish(s *session, kind, id string, wantStatus int) map[string]any {
	h.t.Helper()
	version := h.item(s, kind, id)["version"]
	return h.call(req{method: http.MethodPost, path: contentPath(kind, id, "publish"), body: map[string]any{"base_version": version}, sess: s}, wantStatus)
}

// Stage 5 done-when, adapted to DQ-14: a Content Editor drafts a programme
// and a publisher (Registrar) publishes it, with no developer involved.
func TestProgrammeEditorialWorkflow(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	h.createUser("editor@example.invalid", "content_editor")
	h.createUser("registrar@example.invalid", "registrar")
	editor := h.mustLogin("editor@example.invalid")
	registrar := h.mustLogin("registrar@example.invalid")

	schoolID := h.create(editor, "schools", schoolDoc("testing"))
	h.publish(registrar, "schools", schoolID, http.StatusOK)

	progID := h.create(editor, "programmes", programmeDoc(schoolID, "diploma-in-testing"))
	draft := h.item(editor, "programmes", progID)
	if draft["status"] != "draft" || draft["pending"] != nil {
		t.Fatalf("new programme: %v", draft)
	}

	// The editor cannot publish; the registrar can, once it is complete.
	expectStatus(t, h.do(req{method: http.MethodPost, path: contentPath("programmes", progID, "publish"), body: map[string]any{"base_version": draft["version"]}, sess: editor}), http.StatusForbidden, "forbidden")

	incomplete := programmeDoc(schoolID, "incomplete")
	incomplete["fees"], incomplete["requirements"] = []any{}, []any{}
	incompleteID := h.create(editor, "programmes", incomplete)
	w := h.publish(registrar, "programmes", incompleteID, http.StatusUnprocessableEntity)
	fields := w["error"].(map[string]any)["fields"].([]any)
	if len(fields) != 2 {
		t.Fatalf("missing items for incomplete programme: %v", fields)
	}

	published := h.publish(registrar, "programmes", progID, http.StatusOK)
	if published["status"] != "published" || published["published_at"] == nil {
		t.Fatalf("after publish: %v", published)
	}
	live, err := dbq.New(h.pool).ListPublishedProgrammes(ctx)
	if err != nil || len(live) != 1 || live[0].Title != "Diploma in Testing" {
		t.Fatalf("published programmes = %v, %v", live, err)
	}
	intakeID := published["live"].(map[string]any)["intakes"].([]any)[0].(map[string]any)["id"]

	// Editing a published programme does not change the live site.
	edited := programmeDoc(schoolID, "diploma-in-testing")
	edited["title"] = "Diploma in Advanced Testing"
	edited["fees"].([]any)[0].(map[string]any)["amount_cents"] = 5000000
	edited["intakes"].([]any)[0].(map[string]any)["id"] = intakeID
	after := h.call(req{method: http.MethodPut, path: contentPath("programmes", progID), body: map[string]any{"document": edited, "base_version": published["version"]}, sess: editor}, http.StatusOK)
	if after["pending"] == nil || after["live"].(map[string]any)["title"] != "Diploma in Testing" {
		t.Fatalf("edit of published programme: %v", after)
	}
	var liveFee int64
	_ = h.pool.QueryRow(ctx, `SELECT amount_cents FROM fee_items WHERE programme_id = $1`, progID).Scan(&liveFee)
	if liveFee != 4500000 {
		t.Fatalf("live fee changed to %d before approval", liveFee)
	}

	// A second save based on the old version is refused.
	expectStatus(t, h.do(req{method: http.MethodPut, path: contentPath("programmes", progID), body: map[string]any{"document": edited, "base_version": published["version"]}, sess: editor}), http.StatusConflict, "edit_conflict")

	republished := h.publish(registrar, "programmes", progID, http.StatusOK)
	if republished["pending"] != nil || republished["live"].(map[string]any)["title"] != "Diploma in Advanced Testing" {
		t.Fatalf("after approving the edit: %v", republished)
	}
	_ = h.pool.QueryRow(ctx, `SELECT amount_cents FROM fee_items WHERE programme_id = $1`, progID).Scan(&liveFee)
	if liveFee != 5000000 {
		t.Fatalf("approved fee is %d", liveFee)
	}
	if got := republished["live"].(map[string]any)["intakes"].([]any)[0].(map[string]any)["id"]; got != intakeID {
		t.Fatalf("intake id changed from %v to %v; applications would lose their intake", intakeID, got)
	}

	// Published content is archived, never deleted.
	expectStatus(t, h.do(req{method: http.MethodDelete, path: contentPath("programmes", progID), sess: editor}), http.StatusConflict, "not_deletable")
	h.call(req{method: http.MethodPost, path: contentPath("programmes", progID, "archive"), sess: registrar}, http.StatusOK)
	if live, _ := dbq.New(h.pool).ListPublishedProgrammes(ctx); len(live) != 0 {
		t.Fatal("archived programme still listed as published")
	}
	h.call(req{method: http.MethodDelete, path: contentPath("programmes", incompleteID), sess: editor}, http.StatusNoContent)

	for action, want := range map[string]int{"content.created": 3, "content.published": 3, "content.updated": 1, "content.archived": 1, "content.deleted": 1} {
		if got := h.auditCount(action); got != want {
			t.Errorf("%s audited %d times, want %d", action, got, want)
		}
	}
}

func pngBytes(t *testing.T, w, hgt int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, hgt))
	for x := 0; x < w; x++ {
		img.Set(x, hgt/2, color.RGBA{R: 200, A: 255})
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func (h *harness) upload(s *session, filename string, data []byte) *httptestResult {
	h.t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("file", filename)
	_, _ = fw.Write(data)
	_ = mw.Close()
	w := h.do(req{method: http.MethodPost, path: "/api/v1/staff/images", raw: &body, contentType: mw.FormDataContentType(), sess: s})
	out := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return &httptestResult{code: w.Code, body: out}
}

type httptestResult struct {
	code int
	body map[string]any
}

func TestImageUpload(t *testing.T) {
	h := newHarness(t)
	h.createUser("editor@example.invalid", "content_editor")
	editor := h.mustLogin("editor@example.invalid")

	res := h.upload(editor, "campus.png", pngBytes(t, 2000, 1000))
	if res.code != http.StatusCreated {
		t.Fatalf("upload: %d %v", res.code, res.body)
	}
	variants := res.body["variants"].([]any)
	if len(variants) != 3 || res.body["width"].(float64) != 1600 || res.body["height"].(float64) != 800 {
		t.Fatalf("variants %v, size %vx%v", variants, res.body["width"], res.body["height"])
	}
	if !strings.HasPrefix(res.body["url"].(string), "https://cdn.example/dakota-public/images/") || len(h.storage.Objects) != 3 {
		t.Fatalf("url %v, %d stored objects", res.body["url"], len(h.storage.Objects))
	}
	got := h.call(req{method: http.MethodGet, path: "/api/v1/staff/images/" + res.body["id"].(string), sess: editor}, http.StatusOK)
	if got["url"] != res.body["url"] {
		t.Fatalf("GET image = %v", got)
	}

	for name, data := range map[string][]byte{
		"notes.png":  []byte("this is a text file, not an image"),
		"broken.png": pngBytes(t, 10, 10)[:40],
	} {
		r := h.upload(editor, name, data)
		if r.code != http.StatusUnprocessableEntity || r.body["error"].(map[string]any)["code"] != "invalid_image" {
			t.Errorf("%s: %d %v", name, r.code, r.body)
		}
	}
	if h.auditCount("file.uploaded") != 1 {
		t.Fatal("upload not audited")
	}
}

// Every content type goes through create, edit, publish, pending edit,
// approve, unpublish and archive, by a Super Admin (who holds every
// content permission).
func TestEveryContentKindWorkflow(t *testing.T) {
	h := newHarness(t)
	h.firstSuperAdmin("owner@example.invalid")
	owner := h.mustLogin("owner@example.invalid")

	schoolID := h.create(owner, "schools", schoolDoc("kinds"))
	h.publish(owner, "schools", schoolID, http.StatusOK)
	categoryID := h.create(owner, "faq-categories", map[string]any{"slug": "admissions", "name": "Admissions", "sort_order": 0})
	img := h.upload(owner, "photo.png", pngBytes(t, 800, 600))
	now := time.Now().UTC()

	docs := map[string]map[string]any{
		"programmes": programmeDoc(schoolID, "kinds-programme"),
		"news": {"slug": "open-day", "title": "Open day", "summary": "Visit us.", "body": "Details.", "cover_file_id": img.body["id"],
			"cover_alt": "The college gate", "seo_title": "", "seo_description": ""},
		"events": {"slug": "open-day-event", "title": "Open day", "summary": "", "body": "", "location": "Campus",
			"starts_at": now.Add(48 * time.Hour).Format(time.RFC3339), "ends_at": now.Add(52 * time.Hour).Format(time.RFC3339)},
		"faqs": {"category_id": categoryID, "programme_id": nil, "question": "How do I apply?", "answer": "Online.", "sort_order": 0},
		"banners": {"message": "Applications are open.", "link_label": "Apply", "link_href": "/apply",
			"starts_at": now.Format(time.RFC3339), "ends_at": now.Add(240 * time.Hour).Format(time.RFC3339)},
		"gallery":    {"file_id": img.body["id"], "caption": "", "alt_text": "Students in the library", "consent_confirmed": true, "consent_note": "Form on file.", "sort_order": 0},
		"statistics": {"label": "Graduates", "value": "120", "source_note": "Registry report 2026", "verified_on": now.Format(time.DateOnly), "sort_order": 0},
		"accreditations": {"body": "Test Authority", "reference": "TEST-001", "description": "", "evidence_note": "Certificate on file",
			"verified_on": now.Format(time.DateOnly), "sort_order": 0},
		"pages": {"slug": "student-life", "title": "Student life", "summary": "", "body": "## Clubs\n\n- Drama", "seo_title": "", "seo_description": ""},
	}
	for kind, doc := range docs {
		t.Run(kind, func(t *testing.T) {
			id := h.create(owner, kind, doc)
			h.publish(owner, kind, id, http.StatusOK)
			cur := h.item(owner, kind, id)
			h.call(req{method: http.MethodPut, path: contentPath(kind, id), body: map[string]any{"document": doc, "base_version": cur["version"]}, sess: owner}, http.StatusOK)
			if h.item(owner, kind, id)["pending"] == nil {
				t.Fatal("edit of published item did not become pending")
			}
			h.call(req{method: http.MethodDelete, path: contentPath(kind, id, "pending"), sess: owner}, http.StatusNoContent)
			h.publish(owner, kind, id, http.StatusOK)
			if got := h.call(req{method: http.MethodPost, path: contentPath(kind, id, "unpublish"), sess: owner}, http.StatusOK); got["status"] != "draft" {
				t.Fatalf("unpublish: %v", got)
			}
			if got := h.call(req{method: http.MethodPost, path: contentPath(kind, id, "archive"), sess: owner}, http.StatusOK); got["status"] != "archived" {
				t.Fatalf("archive: %v", got)
			}
			list := h.call(req{method: http.MethodGet, path: contentPath(kind), sess: owner}, http.StatusOK)
			if len(list["items"].([]any)) == 0 {
				t.Fatal("list is empty")
			}
		})
	}
}

func TestContentRules(t *testing.T) {
	h := newHarness(t)
	h.createUser("editor@example.invalid", "content_editor")
	h.createUser("registrar@example.invalid", "registrar")
	editor := h.mustLogin("editor@example.invalid")
	registrar := h.mustLogin("registrar@example.invalid")

	// Unverified statistics and unconsented photos cannot be published (NN-01, NN-08).
	statID := h.create(editor, "statistics", map[string]any{"label": "Pass rate", "value": "95%", "source_note": "", "verified_on": nil, "sort_order": 0})
	fields := h.publish(registrar, "statistics", statID, http.StatusUnprocessableEntity)["error"].(map[string]any)["fields"].([]any)
	if len(fields) != 2 {
		t.Fatalf("unverified statistic problems: %v", fields)
	}
	future := time.Now().Add(72 * time.Hour).Format(time.DateOnly)
	expectStatus(t, h.do(req{method: http.MethodPost, path: contentPath("statistics"), body: map[string]any{"document": map[string]any{"label": "x", "value": "1", "source_note": "y", "verified_on": future, "sort_order": 0}}, sess: editor}), http.StatusUnprocessableEntity, "validation_failed")

	img := h.upload(editor, "p.png", pngBytes(t, 100, 100))
	galleryID := h.create(editor, "gallery", map[string]any{"file_id": img.body["id"], "caption": "", "alt_text": "A class", "consent_confirmed": false, "consent_note": "", "sort_order": 0})
	h.publish(registrar, "gallery", galleryID, http.StatusUnprocessableEntity)

	// Bad input says what to fix; unknown fields are refused.
	bad := h.call(req{method: http.MethodPost, path: contentPath("schools"), body: map[string]any{"document": map[string]any{"slug": "Bad Slug!", "name": "", "short_name": "x", "accent_key": "purple", "summary": "", "description": "", "head_message": "", "sort_order": 0}}, sess: editor}, http.StatusUnprocessableEntity)
	if n := len(bad["error"].(map[string]any)["fields"].([]any)); n != 3 {
		t.Fatalf("expected 3 field problems, got %v", bad)
	}
	extra := schoolDoc("extra")
	extra["unexpected"] = true
	expectStatus(t, h.do(req{method: http.MethodPost, path: contentPath("schools"), body: map[string]any{"document": extra}, sess: editor}), http.StatusUnprocessableEntity, "validation_failed")
	h.create(editor, "schools", schoolDoc("dupe"))
	expectStatus(t, h.do(req{method: http.MethodPost, path: contentPath("schools"), body: map[string]any{"document": schoolDoc("dupe")}, sess: editor}), http.StatusConflict, "duplicate_slug")
	expectStatus(t, h.do(req{method: http.MethodGet, path: contentPath("no-such-kind"), sess: editor}), http.StatusNotFound, "not_found")

	// FAQ categories have no draft stage, so only publishers change them.
	expectStatus(t, h.do(req{method: http.MethodPost, path: contentPath("faq-categories"), body: map[string]any{"document": map[string]any{"slug": "fees", "name": "Fees", "sort_order": 0}}, sess: editor}), http.StatusForbidden, "needs_publisher")

	// Site settings: publishers only, validated, empty allowed (hides the field).
	settings := map[string]any{"values": map[string]string{"contact.email": "admissions@college.example", "contact.phone": "+254 712 000 000", "about.motto": ""}}
	expectStatus(t, h.do(req{method: http.MethodPut, path: "/api/v1/staff/settings", body: settings, sess: editor}), http.StatusForbidden, "forbidden")
	expectStatus(t, h.do(req{method: http.MethodPut, path: "/api/v1/staff/settings", body: map[string]any{"values": map[string]string{"contact.email": "not an email", "no.such_key": "x"}}, sess: registrar}), http.StatusUnprocessableEntity, "validation_failed")
	h.call(req{method: http.MethodPut, path: "/api/v1/staff/settings", body: settings, sess: registrar}, http.StatusNoContent)
	list := h.call(req{method: http.MethodGet, path: "/api/v1/staff/settings", sess: editor}, http.StatusOK)
	found := false
	for _, s := range list["settings"].([]any) {
		m := s.(map[string]any)
		if m["key"] == "contact.email" && m["value"] == "admissions@college.example" {
			found = true
		}
	}
	if !found || h.auditCount("settings.updated") != 1 {
		t.Fatal("setting not saved or not audited")
	}
}
