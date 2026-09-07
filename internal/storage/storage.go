package storage

import (
	"context"

	"github.com/google/uuid"

	"papertrail/internal/document"
)

type Storage interface {
	CreateDocument(ctx context.Context, doc document.Document, fingerprint string) (uuid.UUID, error)
	GetDocument(ctx context.Context, id uuid.UUID) (*document.Document, error)
	GetDocumentByPath(ctx context.Context, sourcePath string) (*document.Document, error)
	UpdateDocument(ctx context.Context, id uuid.UUID, doc document.Document, fingerprint string) error
	DeleteDocument(ctx context.Context, id uuid.UUID) error
	ListDocuments(ctx context.Context) ([]document.Document, error)
	GetDocumentsByAuthor(ctx context.Context, author string) ([]uuid.UUID, error)
	GetDocumentsByCreator(ctx context.Context, creator string) ([]uuid.UUID, error)

	GetOrCreateEntity(ctx context.Context, entity document.Entity) (uuid.UUID, error)
	AttachEntityToDocument(ctx context.Context, documentID uuid.UUID, entityID uuid.UUID, role string) error
	GetDocumentEntities(ctx context.Context, documentID uuid.UUID) ([]document.Entity, error)
	GetDocumentEntityIDs(ctx context.Context, documentID uuid.UUID) ([]uuid.UUID, error)
	GetDocumentsByEntity(ctx context.Context, entityID uuid.UUID) ([]uuid.UUID, error)

	CreateRelationship(
		ctx context.Context,
		documentA uuid.UUID,
		documentB uuid.UUID,
		relationshipType string,
		confidence *float64,
	) (uuid.UUID, error)

	GetDocumentRelationships(ctx context.Context, documentID uuid.UUID) ([]DocumentRelationship, error)
	AttachEntityToRelationship(ctx context.Context, relationshipID uuid.UUID, entityID uuid.UUID) error

	Close() error
}

type DocumentRelationship struct {
	ID               uuid.UUID
	DocumentA        uuid.UUID
	DocumentB        uuid.UUID
	RelationshipType string
	Confidence       *float64
	Entities         []document.Entity
}
