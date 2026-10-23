package application

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/auth"
	"github.com/MboyaDan/dakota-college/backend/internal/content"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
	"github.com/MboyaDan/dakota-college/backend/internal/media"
)

// Documents (brief 8, AC-05): ID or birth certificate, certificates and a
// passport photo. They are checked by content (PDF, JPG or PNG), scanned
// when a scanner is configured, kept in the private bucket and reached only
// through short-lived links issued to one signed-in account.

// PermView lets staff open documents of submitted applications.
const PermView = "admissions.view"

const (
	MaxDocumentBytes     = media.MaxUploadBytes
	MaxDocuments         = 12
	FileLinkTTL          = 5 * time.Minute
	DocumentUploadsLimit = MaxDocumentBytes + 1<<20
)

var (
	ErrStorageOff = errors.New("document storage is not configured")
	// ErrRejectedFile means the scanner found malware; the file is not kept.
	ErrRejectedFile = errors.New("file rejected by the virus scanner")
	ErrScanFailed   = errors.New("virus scanner unavailable")
	ErrLinkInvalid  = errors.New("file link not valid")
)

// DocumentKinds and how the form names them.
var DocumentKinds = map[string]string{
	"national_id":       "national ID card",
	"birth_certificate": "birth certificate",
	"passport":          "passport",
	"kcse_certificate":  "KCSE certificate or result slip",
	"other_certificate": "other certificate",
	"passport_photo":    "passport-size photo",
	"other":             "other document",
}

type Document struct {
	ID          uuid.UUID `json:"id"`
	Kind        string    `json:"kind"`
	Name        string    `json:"name"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	ScanStatus  string    `json:"scan_status"`
	CreatedAt   time.Time `json:"created_at"`
}

// requiredDocuments: the identity document chosen in "About you", a
// passport photo, and the KCSE certificate when KCSE is listed (A-21).
func (d *Detail) requiredDocuments() []string {
	var out []string
	if t := d.Personal.IdentityDocumentType; t != "" {
		out = append(out, t)
	}
	out = append(out, "passport_photo")
	if slices.ContainsFunc(d.Education, func(e Education) bool { return e.Level == "kcse" }) {
		out = append(out, "kcse_certificate")
	}
	return out
}

func (d *Detail) missingDocuments(add func(step, field, msg string)) {
	if d.Personal.IdentityDocumentType == "" {
		add(StepDocuments, "identity", "Upload your ID card, birth certificate or passport (choose which in About you).")
	}
	for _, k := range d.requiredDocuments() {
		if !slices.ContainsFunc(d.Documents, func(doc Document) bool { return doc.Kind == k }) {
			add(StepDocuments, k, "Upload your "+DocumentKinds[k]+".")
		}
	}
}

// checkDocument decides what is stored: PDFs as they are, photos and scans
// re-encoded upright without metadata (no GPS location).
func checkDocument(data []byte) (contentType, ext string, out []byte, err error) {
	if len(data) == 0 {
		return "", "", nil, content.Problems{{Field: "file", Message: "The file is empty. Choose the file again."}}.Err()
	}
	if len(data) > MaxDocumentBytes {
		return "", "", nil, content.Problems{{Field: "file", Message: "The file is larger than 10 MB. Take a smaller photo or save the PDF at a lower quality."}}.Err()
	}
	switch http.DetectContentType(data) {
	case "application/pdf":
		if !bytes.HasPrefix(data, []byte("%PDF-")) {
			break
		}
		return "application/pdf", "pdf", data, nil
	case "image/jpeg", "image/png":
		ct, ext, out, err := media.ProcessDocumentImage(data)
		if err != nil {
			return "", "", nil, content.Problems{{Field: "file", Message: "The image could not be read. Take the photo again, or use a PDF."}}.Err()
		}
		return ct, ext, out, nil
	}
	return "", "", nil, content.Problems{{Field: "file", Message: "Use a PDF, JPG or PNG file."}}.Err()
}

func cleanFilename(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '"' {
			return -1
		}
		return r
	}, name)
	if len([]rune(name)) > 120 {
		name = string([]rune(name)[:120])
	}
	if name == "" || name == "." || name == "/" {
		name = "document"
	}
	return name
}

// UploadDocument checks, scans and stores a document on the user's draft.
func (s *Service) UploadDocument(ctx context.Context, p *auth.Principal, id uuid.UUID, kind, filename string, data []byte, req audit.Request) (*Document, error) {
	if s.storage == nil {
		return nil, ErrStorageOff
	}
	// Check ownership and status before storing anything.
	a, err := s.load(ctx, p.UserID, id)
	if err != nil {
		return nil, err
	}
	if a.Status != "draft" {
		return nil, ErrNotEditable
	}
	if _, ok := DocumentKinds[kind]; !ok {
		return nil, content.Problems{{Field: "kind", Message: "Choose which document this is."}}.Err()
	}
	ct, ext, stored, err := checkDocument(data)
	if err != nil {
		return nil, err
	}
	// Scan what the applicant sent, before anything is kept.
	status, err := s.scanner.Scan(ctx, data)
	if err != nil {
		s.logger.Error("document scan failed", slog.String("error", err.Error()))
		return nil, ErrScanFailed
	}
	if status == media.ScanInfected {
		s.logger.Warn("document rejected by scanner", slog.String("application_id", id.String()))
		return nil, ErrRejectedFile
	}

	key := fmt.Sprintf("applications/%s/%s.%s", id, uuid.NewString(), ext)
	if err := s.storage.PutPrivate(ctx, s.bucket, key, stored, ct); err != nil {
		return nil, fmt.Errorf("store document: %w", err)
	}
	sum := sha256.Sum256(stored)
	name := cleanFilename(filename)
	if !strings.HasSuffix(strings.ToLower(name), "."+ext) {
		name = strings.TrimSuffix(name, path.Ext(name)) + "." + ext
	}
	var doc Document
	err = s.edit(ctx, p.UserID, id, func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error {
		existing, err := q.ListApplicationDocuments(ctx, a.ID)
		if err != nil {
			return err
		}
		if len(existing) >= MaxDocuments {
			return content.Problems{{Field: "file", Message: fmt.Sprintf("You can upload at most %d documents. Remove one first.", MaxDocuments)}}.Err()
		}
		fileID, err := q.InsertPrivateFile(ctx, dbq.InsertPrivateFileParams{
			Bucket: s.bucket, ObjectKey: key, OriginalName: name, ContentType: ct, SizeBytes: int64(len(stored)),
			Sha256: sum[:], ScanStatus: status, UploadedByUserID: &p.UserID,
		})
		if err != nil {
			return err
		}
		docID, err := q.InsertApplicationDocument(ctx, dbq.InsertApplicationDocumentParams{ApplicationID: a.ID, Kind: kind, FileID: fileID})
		if err != nil {
			return err
		}
		doc = Document{ID: docID, Kind: kind, Name: name, ContentType: ct, SizeBytes: int64(len(stored)), ScanStatus: status, CreatedAt: s.now()}
		return audit.Write(ctx, q, req, audit.Entry{Action: "application.document_uploaded", EntityType: "application", EntityID: a.ID.String(),
			Details: map[string]any{"kind": kind, "bytes": len(stored), "scan": status}})
	})
	if err != nil {
		// Nothing points at the object; remove it.
		if derr := s.storage.Delete(context.WithoutCancel(ctx), s.bucket, key); derr != nil {
			s.logger.Error("could not remove unused upload", slog.String("key", key), slog.String("error", derr.Error()))
		}
		return nil, err
	}
	return &doc, nil
}

// RemoveDocument takes a document off a draft. The file row is kept, marked
// deleted, for the audit trail; the stored object is removed.
func (s *Service) RemoveDocument(ctx context.Context, userID, id, docID uuid.UUID, req audit.Request) error {
	var bucket, key string
	err := s.edit(ctx, userID, id, func(q *dbq.Queries, a dbq.LockOwnApplicationRow) error {
		d, err := q.GetApplicationDocument(ctx, dbq.GetApplicationDocumentParams{ID: docID, ApplicationID: a.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if err := q.DeleteApplicationDocument(ctx, d.ID); err != nil {
			return err
		}
		if err := q.MarkFileDeleted(ctx, d.FileID); err != nil {
			return err
		}
		bucket, key = d.Bucket, d.ObjectKey
		return audit.Write(ctx, q, req, audit.Entry{Action: "application.document_removed", EntityType: "application", EntityID: a.ID.String(),
			Details: map[string]any{"document_id": d.ID.String()}})
	})
	if err == nil && s.storage != nil {
		if derr := s.storage.Delete(ctx, bucket, key); derr != nil {
			s.logger.Error("could not remove deleted document", slog.String("key", key), slog.String("error", derr.Error()))
		}
	}
	return err
}

// FileLink is a short-lived address for one file, usable only by the
// account it was issued to.
type FileLink struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Service) issueLink(ctx context.Context, q *dbq.Queries, fileID, userID uuid.UUID) (*FileLink, error) {
	token, hash := auth.NewToken()
	expires := s.now().Add(FileLinkTTL)
	if err := q.CreateFileLink(ctx, dbq.CreateFileLinkParams{TokenHash: hash, FileID: fileID, IssuedTo: userID, ExpiresAt: expires}); err != nil {
		return nil, err
	}
	return &FileLink{URL: "/api/v1/files/" + token, ExpiresAt: expires}, nil
}

// DocumentLink gives the applicant a link to one of their own documents.
func (s *Service) DocumentLink(ctx context.Context, userID, id, docID uuid.UUID) (*FileLink, error) {
	if _, err := s.load(ctx, userID, id); err != nil {
		return nil, err
	}
	q := dbq.New(s.pool)
	d, err := q.GetApplicationDocument(ctx, dbq.GetApplicationDocumentParams{ID: docID, ApplicationID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.issueLink(ctx, q, d.FileID, userID)
}

// StaffDocumentLink gives a member of staff with admissions.view (checked by
// the route) a link to a document of a submitted application. Audited.
func (s *Service) StaffDocumentLink(ctx context.Context, p *auth.Principal, id, docID uuid.UUID, req audit.Request) (*FileLink, error) {
	var link *FileLink
	err := s.tx(ctx, func(q *dbq.Queries) error {
		d, err := q.GetSubmittedApplicationDocument(ctx, dbq.GetSubmittedApplicationDocumentParams{ID: docID, ApplicationID: id})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if link, err = s.issueLink(ctx, q, d.FileID, p.UserID); err != nil {
			return err
		}
		return audit.Write(ctx, q, req, audit.Entry{Action: "file.link_issued", EntityType: "application", EntityID: id.String(),
			Details: map[string]any{"document_id": d.ID.String(), "reference": d.Reference}})
	})
	return link, err
}

// OpenedFile is a file being sent through a link.
type OpenedFile struct {
	Name        string
	ContentType string
	Size        int64
	Body        io.ReadCloser
}

// OpenLink returns the file behind a link if it is still valid and was
// issued to this account. Every failure looks the same to the caller.
func (s *Service) OpenLink(ctx context.Context, userID uuid.UUID, token string) (*OpenedFile, error) {
	if s.storage == nil || token == "" || len(token) > 128 {
		return nil, ErrLinkInvalid
	}
	l, err := dbq.New(s.pool).GetFileLink(ctx, auth.HashToken(token))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrLinkInvalid
	}
	if err != nil {
		return nil, err
	}
	if l.IssuedTo != userID || !s.now().Before(l.ExpiresAt) || l.DeletedAt != nil {
		return nil, ErrLinkInvalid
	}
	body, err := s.storage.Get(ctx, l.Bucket, l.ObjectKey)
	if errors.Is(err, media.ErrObjectNotFound) {
		return nil, ErrLinkInvalid
	}
	if err != nil {
		return nil, err
	}
	return &OpenedFile{Name: l.OriginalName, ContentType: l.ContentType, Size: l.SizeBytes, Body: body}, nil
}
