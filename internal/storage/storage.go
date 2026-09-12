package storage

import (
	"context"
	"time"

	"github.com/google/uuid"

	"papertrail/internal/document"
)

const (
	IntelligenceStatusPending = "pending"
	IntelligenceStatusReady   = "ready"
)

type Storage interface {
	// Document management
	CreateDocument(ctx context.Context, doc document.Document, fingerprint string) (uuid.UUID, error)
	GetDocument(ctx context.Context, id uuid.UUID) (*document.Document, error)
	GetDocumentByPath(ctx context.Context, sourcePath string) (*document.Document, error)
	UpdateDocument(ctx context.Context, id uuid.UUID, doc document.Document, fingerprint string) error
	UpdateDocumentIntelligence(ctx context.Context, id uuid.UUID, metadata document.DocumentMetadata) error
	CommitDocumentIntelligence(ctx context.Context, id uuid.UUID, fingerprint string, result IntelligenceResult) error
	SetDocumentIntelligenceStatus(ctx context.Context, id uuid.UUID, status string) error
	DeleteDocument(ctx context.Context, id uuid.UUID) error
	ListDocuments(ctx context.Context) ([]document.Document, error)
	GetDocumentsByAuthor(ctx context.Context, author string) ([]uuid.UUID, error)
	GetDocumentsByCreator(ctx context.Context, creator string) ([]uuid.UUID, error)
	GetDocumentIndexState(ctx context.Context, sourcePath string) (*DocumentIndexState, error)
	GetDocumentIndexStateByID(ctx context.Context, id uuid.UUID) (*DocumentIndexState, error)

	// Entity management
	GetOrCreateEntity(ctx context.Context, entity document.Entity) (uuid.UUID, error)
	AttachEntityToDocument(ctx context.Context, documentID uuid.UUID, entityID uuid.UUID, role string) error
	ReplaceDocumentEntities(ctx context.Context, documentID uuid.UUID, entities []document.Entity) error
	GetDocumentEntities(ctx context.Context, documentID uuid.UUID) ([]document.Entity, error)
	GetDocumentEntityIDs(ctx context.Context, documentID uuid.UUID) ([]uuid.UUID, error)
	GetDocumentsByEntity(ctx context.Context, entityID uuid.UUID) ([]uuid.UUID, error)
	GetEntity(ctx context.Context, entityID uuid.UUID) (*document.Entity, error)

	// Document relationships
	CreateRelationship(ctx context.Context, documentA uuid.UUID, documentB uuid.UUID, relationshipType string, confidence *float64) (uuid.UUID, error)
	DeleteDocumentRelationships(ctx context.Context, documentID uuid.UUID) error
	GetDocumentRelationships(ctx context.Context, documentID uuid.UUID) ([]DocumentRelationship, error)
	AttachEntityToRelationship(ctx context.Context, relationshipID uuid.UUID, entityID uuid.UUID) error

	// Chunk management
	CreateChunks(ctx context.Context, documentID uuid.UUID, chunks []Chunk) error
	GetChunks(ctx context.Context, documentID uuid.UUID) ([]Chunk, error)
	DeleteDocumentChunks(ctx context.Context, documentID uuid.UUID) error
	ReplaceDocumentChunks(ctx context.Context, documentID uuid.UUID, fingerprint string, chunks []Chunk) error
	SearchChunks(ctx context.Context, embedding []float32, limit int) ([]ChunkSearchResult, error)

	Close() error
}

type DocumentRelationship = document.DocumentRelationship

type DocumentIndexState struct {
	ID                 uuid.UUID
	Size               int64
	ModifiedAt         time.Time
	Fingerprint        string
	IntelligenceStatus string
}

type Chunk struct {
	ID         int64
	DocumentID uuid.UUID
	ChunkIndex int
	Text       string
	Embedding  []float32
}

type ChunkSearchResult struct {
	Chunk      Chunk
	Similarity float32
}

type IntelligenceResult struct {
	Metadata document.DocumentMetadata
	Chunks   []Chunk
}
