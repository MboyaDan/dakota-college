package content

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/MboyaDan/dakota-college/backend/internal/db/dbq"
)

// Document is the editable form of one content item.
type Document interface {
	// Validate checks the document can be saved (as a draft or pending edit).
	Validate() error
}

// State is the workflow state of an item, read from its own table.
type State struct {
	Status      string     `json:"status"`
	PublishedAt *time.Time `json:"published_at"`
	IsSample    bool       `json:"is_sample"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Summary struct {
	ID         uuid.UUID `json:"id"`
	Label      string    `json:"label"`
	Context    string    `json:"context,omitempty"`
	Status     string    `json:"status"`
	IsSample   bool      `json:"is_sample"`
	UpdatedAt  time.Time `json:"updated_at"`
	HasPending bool      `json:"has_pending"`
}

// def describes one content type: where it lives and its rules.
type def[D Document] struct {
	name string
	// workflow types have draft/published/archived and pending edits.
	// Others (FAQ categories) change immediately and need content.publish.
	workflow bool
	// readyToPublish lists what is still missing before the item can go live.
	readyToPublish func(ctx context.Context, q *dbq.Queries, d D) error

	list      func(ctx context.Context, q *dbq.Queries) ([]Summary, error)
	load      func(ctx context.Context, q *dbq.Queries, id uuid.UUID) (D, State, error)
	create    func(ctx context.Context, q *dbq.Queries, d D, actor uuid.UUID) (uuid.UUID, error)
	save      func(ctx context.Context, q *dbq.Queries, id uuid.UUID, d D, actor uuid.UUID) error
	setStatus func(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, publishedAt *time.Time, actor uuid.UUID) error
	remove    func(ctx context.Context, q *dbq.Queries, id uuid.UUID) error
}

// kind is a def with its document type erased, so the service and HTTP
// layer can treat every content type the same way.
type kind interface {
	Name() string
	Workflow() bool
	List(ctx context.Context, q *dbq.Queries) ([]Summary, error)
	Load(ctx context.Context, q *dbq.Queries, id uuid.UUID) (Document, State, error)
	Decode(raw []byte) (Document, error)
	Create(ctx context.Context, q *dbq.Queries, d Document, actor uuid.UUID) (uuid.UUID, error)
	Save(ctx context.Context, q *dbq.Queries, id uuid.UUID, d Document, actor uuid.UUID) error
	SetStatus(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, publishedAt *time.Time, actor uuid.UUID) error
	Remove(ctx context.Context, q *dbq.Queries, id uuid.UUID) error
	ReadyToPublish(ctx context.Context, q *dbq.Queries, d Document) error
}

func (k def[D]) Name() string   { return k.name }
func (k def[D]) Workflow() bool { return k.workflow }

func (k def[D]) List(ctx context.Context, q *dbq.Queries) ([]Summary, error) { return k.list(ctx, q) }

func (k def[D]) Load(ctx context.Context, q *dbq.Queries, id uuid.UUID) (Document, State, error) {
	return k.load(ctx, q, id)
}

// Decode parses and validates a document. Unknown fields are refused so a
// typo in the client cannot silently drop data.
func (k def[D]) Decode(raw []byte) (Document, error) {
	var d D
	dec := json.NewDecoder(bytesReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, &ValidationError{Problems: Problems{{Field: "document", Message: "The form data is not in the expected shape: " + err.Error()}}}
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return d, nil
}

func (k def[D]) Create(ctx context.Context, q *dbq.Queries, d Document, actor uuid.UUID) (uuid.UUID, error) {
	return k.create(ctx, q, d.(D), actor)
}

func (k def[D]) Save(ctx context.Context, q *dbq.Queries, id uuid.UUID, d Document, actor uuid.UUID) error {
	return k.save(ctx, q, id, d.(D), actor)
}

func (k def[D]) SetStatus(ctx context.Context, q *dbq.Queries, id uuid.UUID, status string, publishedAt *time.Time, actor uuid.UUID) error {
	if k.setStatus == nil {
		return fmt.Errorf("%s has no workflow", k.name)
	}
	return k.setStatus(ctx, q, id, status, publishedAt, actor)
}

func (k def[D]) Remove(ctx context.Context, q *dbq.Queries, id uuid.UUID) error {
	return k.remove(ctx, q, id)
}

func (k def[D]) ReadyToPublish(ctx context.Context, q *dbq.Queries, d Document) error {
	if k.readyToPublish == nil {
		return nil
	}
	return k.readyToPublish(ctx, q, d.(D))
}
