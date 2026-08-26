package workflow

import (
	"context"

	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/accessibility"
	"benzhi-project-1097d932-5488-4608-b943-e331f2e1eaed/internal/audit"
)

type Mutation struct {
	Result any
	Event  audit.Event
}

type CreateRecord struct {
	Aggregate accessibility.CaseAggregate
	Event     audit.Event
	Result    any
}

type Repository interface {
	Create(context.Context, string, string, CreateRecord) ([]byte, bool, error)
	Update(context.Context, string, string, string, int64, func(*accessibility.CaseAggregate, []audit.Event) (Mutation, error)) ([]byte, bool, error)
	Get(context.Context, string) (accessibility.CaseAggregate, error)
	Events(context.Context, string) ([]audit.Event, error)
	List(context.Context) ([]accessibility.PublicationCase, error)
	VerifyAll(context.Context) error
	VerifyGraph(context.Context, accessibility.CaseAggregate) error
	StoredManifestDigest(context.Context, string) (string, error)
	Close() error
}
