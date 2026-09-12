package relationship

import (
	"context"
	"fmt"
	"strings"
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
	if err := e.processEntityRelationships(ctx, documentID); err != nil {
		return err
	}

	if err := e.processMetadataRelationships(ctx, documentID); err != nil {
		return err
	}

	return nil
}

func (e *Engine) processEntityRelationships(
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

func (e *Engine) processMetadataRelationships(
	ctx context.Context,
	documentID uuid.UUID,
) error {
	doc, err := e.storage.GetDocument(ctx, documentID)
	if err != nil {
		return fmt.Errorf("get document: %w", err)
	}

	if author := strings.TrimSpace(doc.FileMetadata.Author); author != "" {
		if err := e.processMetadataValue(
			ctx,
			documentID,
			author,
			"shared_author",
			e.storage.GetDocumentsByAuthor,
		); err != nil {
			return err
		}
	}

	if creator := strings.TrimSpace(doc.FileMetadata.Creator); creator != "" {
		if err := e.processMetadataValue(
			ctx,
			documentID,
			creator,
			"shared_creator",
			e.storage.GetDocumentsByCreator,
		); err != nil {
			return err
		}
	}

	return nil
}

func (e *Engine) processMetadataValue(
	ctx context.Context,
	documentID uuid.UUID,
	value string,
	relationshipType string,
	getDocuments func(context.Context, string) ([]uuid.UUID, error),
) error {
	otherDocumentIDs, err := getDocuments(ctx, value)
	if err != nil {
		return fmt.Errorf(
			"get documents for %s: %w",
			relationshipType,
			err,
		)
	}

	for _, otherDocumentID := range otherDocumentIDs {
		if otherDocumentID == documentID {
			continue
		}

		if _, err := e.storage.CreateRelationship(
			ctx,
			documentID,
			otherDocumentID,
			relationshipType,
			nil,
		); err != nil {
			return fmt.Errorf(
				"create %s relationship: %w",
				relationshipType,
				err,
			)
		}
	}

	return nil
}
