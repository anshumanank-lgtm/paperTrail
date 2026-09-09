package search

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/google/uuid"

	"papertrail/internal/document"
)

type Repository interface {
	SearchByFilename(ctx context.Context, value string) ([]document.Document, error)
	SearchByDate(ctx context.Context, value string) ([]document.Document, error)
	SearchByAuthor(ctx context.Context, value string) ([]document.Document, error)
	SearchByCreator(ctx context.Context, value string) ([]document.Document, error)
	SearchByText(ctx context.Context, value string) ([]document.DocumentTextMatch, error)
	SearchByType(ctx context.Context, value string) ([]document.Document, error)

	GetDocument(ctx context.Context, id uuid.UUID) (*document.Document, error)
	GetDocumentEntities(ctx context.Context, documentID uuid.UUID) ([]document.Entity, error)
	GetDocumentRelationships(ctx context.Context, documentID uuid.UUID) ([]document.DocumentRelationship, error)
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{
		repository: repository,
	}
}

func (s *Service) Search(
	ctx context.Context,
	query SearchQuery,
) ([]SearchResult, error) {
	query.Value = strings.TrimSpace(query.Value)

	log.Printf(
		"[SEARCH] request: by=%s value=%q",
		query.By,
		query.Value,
	)

	if query.Value == "" {
		err := fmt.Errorf("search value is empty")
		log.Printf("[SEARCH] error: %v", err)
		return nil, err
	}

	var (
		documents []document.Document
		matches   map[uuid.UUID][]document.Entity
		err       error
	)

	switch query.By {
	case SearchByFilename:
		documents, err = s.repository.SearchByFilename(ctx, query.Value)

	case SearchByDate:
		documents, err = s.repository.SearchByDate(ctx, query.Value)

	case SearchByAuthor:
		documents, err = s.repository.SearchByAuthor(ctx, query.Value)

	case SearchByCreator:
		documents, err = s.repository.SearchByCreator(ctx, query.Value)

	case SearchByType:
		documents, err = s.repository.SearchByType(ctx, query.Value)

	case SearchByText:
		textMatches, searchErr := s.repository.SearchByText(ctx, query.Value)
		if searchErr != nil {
			log.Printf("[SEARCH] repository error: %v", searchErr)
			return nil, searchErr
		}

		log.Printf(
			"[SEARCH] repository response: text_matches=%d",
			len(textMatches),
		)

		documents = make([]document.Document, 0, len(textMatches))
		matches = make(map[uuid.UUID][]document.Entity)

		for _, match := range textMatches {
			documents = append(documents, match.Document)
			matches[match.Document.ID] = match.Entities
		}

	default:
		err := fmt.Errorf("unsupported search field: %q", query.By)
		log.Printf("[SEARCH] error: %v", err)
		return nil, err
	}

	if err != nil {
		log.Printf("[SEARCH] repository error: %v", err)
		return nil, err
	}

	results := make([]SearchResult, 0, len(documents))

	for _, doc := range documents {
		results = append(results, SearchResult{
			Document:        doc,
			MatchedEntities: matches[doc.ID],
		})
	}

	log.Printf(
		"[SEARCH] response: results=%d",
		len(results),
	)

	for i, result := range results {
		log.Printf(
			"[SEARCH] result[%d]: id=%s title=%q filename=%q type=%s",
			i,
			result.Document.ID,
			result.Document.DocumentMetadata.Title,
			result.Document.FileMetadata.Filename,
			result.Document.DocumentMetadata.DocumentType,
		)
	}

	return results, nil
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

	entities, err := s.repository.GetDocumentEntities(ctx, documentID)
	if err != nil {
		log.Printf("[SEARCH] detail entities error: %v", err)
		return nil, fmt.Errorf("get document entities: %w", err)
	}

	relationships, err := s.repository.GetDocumentRelationships(ctx, documentID)
	if err != nil {
		log.Printf("[SEARCH] detail relationships error: %v", err)
		return nil, fmt.Errorf("get document relationships: %w", err)
	}

	result := &DocumentDetail{
		Document:      *doc,
		Entities:      entities,
		Relationships: relationships,
	}

	log.Printf(
		"[SEARCH] detail response: document_id=%s title=%q entities=%d relationships=%d",
		documentID,
		doc.DocumentMetadata.Title,
		len(entities),
		len(relationships),
	)

	return result, nil
}
