package search

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	"papertrail/internal/document"
	"papertrail/internal/storage"
)

type Repository interface {
	SearchByFilename(ctx context.Context, value string) ([]document.Document, error)
	SearchByDate(ctx context.Context, value string) ([]document.Document, error)
	SearchByAuthor(ctx context.Context, value string) ([]document.Document, error)
	SearchByCreator(ctx context.Context, value string) ([]document.Document, error)
	SearchByText(ctx context.Context, value string) ([]document.DocumentTextMatch, error)
	SearchByType(ctx context.Context, value string) ([]document.Document, error)

	GetDocument(ctx context.Context, id uuid.UUID) (*document.Document, error)
	ListDocuments(ctx context.Context) ([]document.Document, error)
	GetDocumentEntities(ctx context.Context, documentID uuid.UUID) ([]document.Entity, error)
	GetDocumentRelationships(ctx context.Context, documentID uuid.UUID) ([]document.DocumentRelationship, error)

	GetEntity(ctx context.Context, entityID uuid.UUID) (*document.Entity, error)
	GetDocumentsByEntity(ctx context.Context, entityID uuid.UUID) ([]document.Document, error)

	SearchChunks(ctx context.Context, embedding []float32, limit int) ([]storage.ChunkSearchResult, error)
}

type Service struct {
	repository Repository
	embedder   Embedder
	answerer   Answerer
}

type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

type Answerer interface {
	Answer(
		ctx context.Context,
		question string,
		chunks []string,
	) (string, error)
}

func NewService(
	repository Repository,
	embedder Embedder,
	answerer Answerer,
) *Service {
	return &Service{
		repository: repository,
		embedder:   embedder,
		answerer:   answerer,
	}
}

func (s *Service) GetDocumentDetail(
	ctx context.Context,
	documentID uuid.UUID,
) (*DocumentDetail, error) {
	log.Printf(
		"[SEARCH] detail request: document_id=%s",
		documentID,
	)

	doc, err := s.repository.GetDocument(ctx, documentID)
	if err != nil {
		log.Printf("[SEARCH] detail document error: %v", err)
		return nil, err
	}

	entities, err := s.repository.GetDocumentEntities(
		ctx,
		documentID,
	)
	if err != nil {
		log.Printf("[SEARCH] detail entities error: %v", err)
		return nil, fmt.Errorf("get document entities: %w", err)
	}

	relationships, err := s.repository.GetDocumentRelationships(
		ctx,
		documentID,
	)
	if err != nil {
		log.Printf("[SEARCH] detail relationships error: %v", err)
		return nil, fmt.Errorf("get document relationships: %w", err)
	}

	details := make([]RelationshipDetail, 0, len(relationships))

	for _, relationship := range relationships {
		otherDocumentID := relationship.DocumentB
		if relationship.DocumentB == documentID {
			otherDocumentID = relationship.DocumentA
		}

		otherDocument, err := s.repository.GetDocument(
			ctx,
			otherDocumentID,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"get related document %s: %w",
				otherDocumentID,
				err,
			)
		}

		details = append(details, RelationshipDetail{
			Relationship:  relationship,
			OtherDocument: *otherDocument,
		})
	}

	result := &DocumentDetail{
		Document:      *doc,
		Entities:      entities,
		Relationships: details,
	}

	log.Printf(
		"[SEARCH] detail response: document_id=%s title=%q entities=%d relationships=%d",
		documentID,
		doc.DocumentMetadata.Title,
		len(entities),
		len(details),
	)

	return result, nil
}

func (s *Service) ListDocuments(
	ctx context.Context,
) ([]document.Document, error) {
	log.Printf("[SEARCH] list documents request")

	documents, err := s.repository.ListDocuments(ctx)
	if err != nil {
		log.Printf("[SEARCH] list documents error: %v", err)
		return nil, err
	}

	log.Printf(
		"[SEARCH] list documents response: documents=%d",
		len(documents),
	)

	return documents, nil
}

func (s *Service) GetEntityDetail(
	ctx context.Context,
	entityID uuid.UUID,
) (*EntityDetail, error) {
	log.Printf(
		"[SEARCH] entity request: entity_id=%s",
		entityID,
	)

	entity, err := s.repository.GetEntity(ctx, entityID)
	if err != nil {
		log.Printf("[SEARCH] entity error: %v", err)
		return nil, err
	}

	documents, err := s.repository.GetDocumentsByEntity(
		ctx,
		entityID,
	)
	if err != nil {
		log.Printf("[SEARCH] entity documents error: %v", err)
		return nil, fmt.Errorf(
			"get entity documents: %w",
			err,
		)
	}

	return &EntityDetail{
		Entity:    *entity,
		Documents: documents,
	}, nil
}

func (s *Service) Ask(
	ctx context.Context,
	question string,
) (string, error) {
	question = strings.TrimSpace(question)

	if question == "" {
		return "", fmt.Errorf("question is empty")
	}

	log.Printf(
		"[SEARCH] ask request: question=%q",
		question,
	)

	embeddings, err := s.embedder.Embed(
		ctx,
		[]string{question},
	)
	if err != nil {
		return "", fmt.Errorf(
			"embed question: %w",
			err,
		)
	}

	if len(embeddings) != 1 {
		return "", fmt.Errorf(
			"unexpected query embedding count: %d",
			len(embeddings),
		)
	}

	results, err := s.repository.SearchChunks(
		ctx,
		embeddings[0],
		5,
	)
	if err != nil {
		return "", fmt.Errorf(
			"search chunks: %w",
			err,
		)
	}

	if len(results) == 0 {
		return "I couldn't find any relevant information in your documents.", nil
	}

	contextChunks := make([]string, 0, len(results))

	for _, result := range results {
		doc, err := s.repository.GetDocument(
			ctx,
			result.Chunk.DocumentID,
		)
		if err != nil {
			return "", fmt.Errorf(
				"get document metadata: %w",
				err,
			)
		}

		var b strings.Builder

		fmt.Fprintf(
			&b,
			"Document: %s\n",
			doc.DocumentMetadata.Title,
		)

		fmt.Fprintf(
			&b,
			"Filename: %s\n",
			doc.FileMetadata.Filename,
		)

		fmt.Fprintf(
			&b,
			"Document type: %s\n\n",
			doc.DocumentMetadata.DocumentType,
		)

		fmt.Fprintf(
			&b,
			"Content:\n%s",
			result.Chunk.Text,
		)

		contextChunks = append(
			contextChunks,
			b.String(),
		)
	}

	log.Printf(
		"[SEARCH] ask response: context_chunks=%d",
		len(contextChunks),
	)
	return s.answerer.Answer(
		ctx,
		question,
		contextChunks,
	)
}
