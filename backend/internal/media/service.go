package media

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MboyaDan/dakota-college/backend/internal/audit"
	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

var ErrNotFound = errors.New("image not found")

// Images uploads public website images (programme, news, gallery). Private
// applicant documents use a different bucket and signed links (Stage 9).
type Images struct {
	Pool          *pgxpool.Pool
	Storage       Storage
	Bucket        string
	PublicBaseURL string
}

type ImageVariant struct {
	Width int    `json:"width"`
	URL   string `json:"url"`
}

type ImageFile struct {
	ID       uuid.UUID      `json:"id"`
	Width    int            `json:"width"`
	Height   int            `json:"height"`
	URL      string         `json:"url"`
	Variants []ImageVariant `json:"variants"`
}

func variantKey(prefix string, width int, ext string) string {
	return fmt.Sprintf("%s/%d.%s", prefix, width, ext)
}

func (s *Images) describe(id uuid.UUID, objectKey string, width, height int, widths []int32) *ImageFile {
	prefix, file := path.Split(objectKey)
	ext := strings.TrimPrefix(path.Ext(file), ".")
	f := &ImageFile{ID: id, Width: width, Height: height, URL: PublicURL(s.PublicBaseURL, objectKey)}
	for _, w := range widths {
		f.Variants = append(f.Variants, ImageVariant{Width: int(w), URL: PublicURL(s.PublicBaseURL, variantKey(strings.TrimSuffix(prefix, "/"), int(w), ext))})
	}
	return f
}

// Upload validates, re-encodes and stores an image, then records it.
func (s *Images) Upload(ctx context.Context, actor uuid.UUID, filename string, data []byte, req audit.Request) (*ImageFile, error) {
	p, err := Process(data)
	if err != nil {
		return nil, err
	}
	prefix := "images/" + uuid.NewString()
	var widths []int32
	for _, v := range p.Variants {
		if err := s.Storage.Put(ctx, s.Bucket, variantKey(prefix, v.Width, p.Ext), v.Data, p.ContentType); err != nil {
			return nil, fmt.Errorf("store image: %w", err)
		}
		widths = append(widths, int32(v.Width))
	}
	largest := p.Variants[len(p.Variants)-1]
	sum := sha256.Sum256(data)
	name := path.Base(strings.ReplaceAll(filename, "\\", "/"))
	if len(name) > 200 {
		name = name[:200]
	}
	w, h := int32(largest.Width), int32(largest.Height)
	objectKey := variantKey(prefix, largest.Width, p.Ext)

	var id uuid.UUID
	err = pgx.BeginFunc(ctx, s.Pool, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		row, err := q.InsertImageFile(ctx, dbq.InsertImageFileParams{
			Bucket: s.Bucket, ObjectKey: objectKey, OriginalName: name, ContentType: p.ContentType,
			SizeBytes: int64(len(largest.Data)), Sha256: sum[:], UploadedByUserID: &actor, Width: &w, Height: &h, ImageWidths: widths,
		})
		if err != nil {
			return err
		}
		id = row.ID
		return audit.Write(ctx, q, req, audit.Entry{Action: "file.uploaded", EntityType: "file", EntityID: id.String(),
			Details: map[string]any{"kind": "image", "bytes": len(data)}})
	})
	if err != nil {
		return nil, err
	}
	return s.describe(id, objectKey, largest.Width, largest.Height, widths), nil
}

func (s *Images) Get(ctx context.Context, id uuid.UUID) (*ImageFile, error) {
	f, err := dbq.New(s.Pool).GetFile(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && f.Bucket != s.Bucket) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	var w, h int
	if f.Width != nil {
		w = int(*f.Width)
	}
	if f.Height != nil {
		h = int(*f.Height)
	}
	return s.describe(f.ID, f.ObjectKey, w, h, f.ImageWidths), nil
}
