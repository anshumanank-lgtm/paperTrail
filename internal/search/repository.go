package search

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"papertrail/internal/document"
	"papertrail/internal/storage"
)

// StorageRepository adapts storage operations to the search repository contract.
type StorageRepository struct {
	*storage.SQLiteStorage
}

// GetDocumentsByEntity retrieves full documents associated with an entity.
func (s StorageRepository) GetDocumentsByEntity(
	ctx context.Context,
	entityID uuid.UUID,
) ([]document.Document, error) {
	ids, err := s.SQLiteStorage.GetDocumentsByEntity(ctx, entityID)
	if err != nil {
		return nil, err
	}

	documents := make([]document.Document, 0, len(ids))

	for _, id := range ids {
		doc, err := s.GetDocument(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("get document %s: %w", id, err)
		}

		documents = append(documents, *doc)
	}

	return documents, nil
}
