package relationship

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"papertrail/internal/document"
	"papertrail/internal/storage"
)

type Engine struct {
	storage storage.Storage
}

func NewEngine(storage storage.Storage) *Engine {
	return &Engine{
		storage: storage,
	}
}

var relationshipEntityPriority = []document.EntityType{
	document.EntityTypeOrganisation,
	document.EntityTypeProduct,
	document.EntityTypeIDNumber,
	document.EntityTypeEvent,
	document.EntityTypeAddress,
	document.EntityTypePerson,
}

func relationshipType(entityType document.EntityType) (string, bool) {
	switch entityType {
	case document.EntityTypeOrganisation:
		return "shared_organisation", true
	case document.EntityTypeProduct:
		return "shared_product", true
	case document.EntityTypeIDNumber:
		return "shared_id", true
	case document.EntityTypeEvent:
		return "shared_event", true
	case document.EntityTypeAddress:
		return "shared_address", true
	case document.EntityTypePerson:
		return "shared_person", true
	default:
		return "", false
	}
}

type RelatedDocument struct {
	DocumentID       uuid.UUID
	RelationshipType string
	Entities         []document.Entity
	ModifiedAt       time.Time
}

func (e *Engine) ProcessDocument(
	ctx context.Context,
	documentID uuid.UUID,
) error {
	entities, err := e.storage.GetDocumentEntities(
		ctx,
		documentID,
	)
	if err != nil {
		return fmt.Errorf("get document entities: %w", err)
	}

	for _, entity := range entities {
		relationshipType, ok := relationshipType(entity.Type)
		if !ok {
			continue
		}

		otherDocumentIDs, err := e.storage.GetDocumentsByEntity(
			ctx,
			entity.ID,
		)
		if err != nil {
			return fmt.Errorf(
				"get documents by entity: %w",
				err,
			)
		}

		for _, otherDocumentID := range otherDocumentIDs {
			if otherDocumentID == documentID {
				continue
			}

			relationshipID, err := e.storage.CreateRelationship(
				ctx,
				documentID,
				otherDocumentID,
				relationshipType,
				nil,
			)
			if err != nil {
				return fmt.Errorf(
					"create relationship: %w",
					err,
				)
			}

			if err := e.storage.AttachEntityToRelationship(
				ctx,
				relationshipID,
				entity.ID,
			); err != nil {
				return fmt.Errorf(
					"attach relationship entity: %w",
					err,
				)
			}
		}
	}

	return nil
}

func (e *Engine) GetRelatedDocuments(
	ctx context.Context,
	documentID uuid.UUID,
	limit int,
) ([]RelatedDocument, error) {
	if limit <= 0 {
		return []RelatedDocument{}, nil
	}

	relationships, err := e.storage.GetDocumentRelationships(
		ctx,
		documentID,
	)
	if err != nil {
		return nil, fmt.Errorf("get document relationships: %w", err)
	}

	best := make(map[uuid.UUID]storage.DocumentRelationship)

	for _, relationship := range relationships {
		otherDocumentID := relationship.DocumentB

		if otherDocumentID == documentID {
			otherDocumentID = relationship.DocumentA
		}

		priority, ok := relationshipPriority(
			relationship.RelationshipType,
		)
		if !ok {
			continue
		}

		current, exists := best[otherDocumentID]

		if exists {
			currentPriority, _ := relationshipPriority(
				current.RelationshipType,
			)

			if priority >= currentPriority {
				continue
			}
		}

		best[otherDocumentID] = relationship
	}

	results := make([]RelatedDocument, 0, len(best))

	for _, relationship := range best {
		otherDocumentID := relationship.DocumentB

		if otherDocumentID == documentID {
			otherDocumentID = relationship.DocumentA
		}

		doc, err := e.storage.GetDocument(
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

		results = append(results, RelatedDocument{
			DocumentID:       otherDocumentID,
			RelationshipType: relationship.RelationshipType,
			Entities:         relationship.Entities,
			ModifiedAt:       doc.FileMetadata.ModifiedAt,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ModifiedAt.After(results[j].ModifiedAt)
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func relationshipPriority(relationshipType string) (int, bool) {
	switch relationshipType {
	case "shared_organisation":
		return 1, true
	case "shared_product":
		return 2, true
	case "shared_id":
		return 3, true
	case "shared_event":
		return 4, true
	case "shared_address":
		return 5, true
	case "shared_person":
		return 6, true
	default:
		return 0, false
	}
}
