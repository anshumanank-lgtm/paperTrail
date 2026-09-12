package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"papertrail/internal/document"
)

func (s *SQLiteStorage) CommitDocumentIntelligence(
	ctx context.Context,
	documentID uuid.UUID,
	fingerprint string,
	result IntelligenceResult,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin intelligence transaction: %w", err)
	}
	defer tx.Rollback()

	var currentFingerprint string
	if err := tx.QueryRowContext(ctx, `
		SELECT fingerprint
		FROM documents
		WHERE id = ?
	`, uuidToBytes(documentID)).Scan(&currentFingerprint); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("document not found: %s", documentID)
		}
		return fmt.Errorf("get document fingerprint: %w", err)
	}
	if currentFingerprint != fingerprint {
		return ErrStaleDocument
	}

	oldEntityIDs, err := queryDocumentEntityIDs(ctx, tx, documentID)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE documents
		SET title = ?,
		    document_type = ?,
		    intelligence_status = ?,
		    updated_at = ?
		WHERE id = ?
		  AND fingerprint = ?
	`,
		result.Metadata.Title,
		result.Metadata.DocumentType,
		IntelligenceStatusReady,
		time.Now().UTC().Format(time.RFC3339Nano),
		uuidToBytes(documentID),
		fingerprint,
	); err != nil {
		return fmt.Errorf("update document intelligence: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM document_entities
		WHERE document_id = ?
	`, uuidToBytes(documentID)); err != nil {
		return fmt.Errorf("remove existing document entities: %w", err)
	}

	entityIDs := make([]uuid.UUID, 0, len(result.Metadata.Entities))
	for i := range result.Metadata.Entities {
		entity := result.Metadata.Entities[i]
		normalizedValue := normalizeEntityValue(entity.Value)
		if normalizedValue == "" {
			continue
		}

		entityID, err := getOrCreateEntityTx(ctx, tx, entity, normalizedValue)
		if err != nil {
			return fmt.Errorf("store entity %q: %w", entity.Value, err)
		}
		entityIDs = append(entityIDs, entityID)

		if _, err := tx.ExecContext(ctx, `
			INSERT OR IGNORE INTO document_entities (document_id, entity_id, role)
			VALUES (?, ?, ?)
		`, uuidToBytes(documentID), uuidToBytes(entityID), ""); err != nil {
			return fmt.Errorf("attach entity %q: %w", entity.Value, err)
		}
	}

	for _, entityID := range oldEntityIDs {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM entities
			WHERE id = ?
			  AND NOT EXISTS (
				SELECT 1 FROM document_entities WHERE entity_id = entities.id
			  )
		`, uuidToBytes(entityID)); err != nil {
			return fmt.Errorf("cleanup orphan entity %s: %w", entityID, err)
		}
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM document_relationships
		WHERE document_id_a = ?
		   OR document_id_b = ?
	`, uuidToBytes(documentID), uuidToBytes(documentID)); err != nil {
		return fmt.Errorf("remove existing relationships: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM chunks
		WHERE document_id = ?
	`, uuidToBytes(documentID)); err != nil {
		return fmt.Errorf("remove existing chunks: %w", err)
	}

	for _, chunk := range result.Chunks {
		if len(chunk.Embedding) != embeddingDimensions {
			return fmt.Errorf(
				"invalid embedding dimensions for chunk %d: got %d, want %d",
				chunk.ChunkIndex,
				len(chunk.Embedding),
				embeddingDimensions,
			)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO chunks (document_id, chunk_index, text, embedding)
			VALUES (?, ?, ?, ?)
		`,
			uuidToBytes(documentID),
			chunk.ChunkIndex,
			chunk.Text,
			encodeEmbedding(chunk.Embedding),
		); err != nil {
			return fmt.Errorf("insert chunk %d: %w", chunk.ChunkIndex, err)
		}
	}

	if err := createIntelligenceRelationshipsTx(
		ctx,
		tx,
		documentID,
		entityIDs,
	); err != nil {
		return err
	}

	if err := createMetadataRelationshipsTx(ctx, tx, documentID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit intelligence transaction: %w", err)
	}
	return nil
}

func createIntelligenceRelationshipsTx(
	ctx context.Context,
	tx *sql.Tx,
	documentID uuid.UUID,
	entityIDs []uuid.UUID,
) error {
	for _, entityID := range entityIDs {
		var entityType document.EntityType
		if err := tx.QueryRowContext(ctx, `
			SELECT type FROM entities WHERE id = ?
		`, uuidToBytes(entityID)).Scan(&entityType); err != nil {
			return fmt.Errorf("get entity type: %w", err)
		}

		relationshipType, ok := intelligenceRelationshipType(entityType)
		if !ok {
			continue
		}

		rows, err := tx.QueryContext(ctx, `
			SELECT document_id
			FROM document_entities
			WHERE entity_id = ?
		`, uuidToBytes(entityID))
		if err != nil {
			return fmt.Errorf("get related documents: %w", err)
		}

		var otherDocumentIDs []uuid.UUID
		for rows.Next() {
			var idBytes []byte
			if err := rows.Scan(&idBytes); err != nil {
				rows.Close()
				return fmt.Errorf("scan related document: %w", err)
			}
			otherID, err := bytesToUUID(idBytes)
			if err != nil {
				rows.Close()
				return fmt.Errorf("decode related document: %w", err)
			}
			if otherID != documentID {
				otherDocumentIDs = append(otherDocumentIDs, otherID)
			}
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate related documents: %w", err)
		}
		rows.Close()

		for _, otherDocumentID := range otherDocumentIDs {
			relationshipID, err := createRelationshipTx(
				ctx,
				tx,
				documentID,
				otherDocumentID,
				relationshipType,
			)
			if err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `
				INSERT OR IGNORE INTO relationship_entities (relationship_id, entity_id)
				VALUES (?, ?)
			`, uuidToBytes(relationshipID), uuidToBytes(entityID)); err != nil {
				return fmt.Errorf("attach relationship entity: %w", err)
			}
		}
	}
	return nil
}

func createMetadataRelationshipsTx(
	ctx context.Context,
	tx *sql.Tx,
	documentID uuid.UUID,
) error {
	for _, relationship := range []struct {
		column   string
		typeName string
	}{
		{column: "author", typeName: "shared_author"},
		{column: "creator", typeName: "shared_creator"},
	} {
		var value string
		if err := tx.QueryRowContext(ctx, `
			SELECT `+relationship.column+` FROM documents WHERE id = ?
		`, uuidToBytes(documentID)).Scan(&value); err != nil {
			return fmt.Errorf("get document %s: %w", relationship.column, err)
		}
		if value == "" {
			continue
		}

		rows, err := tx.QueryContext(ctx, `
			SELECT id
			FROM documents
			WHERE id != ?
			  AND lower(trim(`+relationship.column+`)) = lower(trim(?))
		`, uuidToBytes(documentID), value)
		if err != nil {
			return fmt.Errorf("get documents for %s: %w", relationship.typeName, err)
		}

		var otherDocumentIDs []uuid.UUID
		for rows.Next() {
			var idBytes []byte
			if err := rows.Scan(&idBytes); err != nil {
				rows.Close()
				return fmt.Errorf("scan %s document: %w", relationship.typeName, err)
			}
			otherID, err := bytesToUUID(idBytes)
			if err != nil {
				rows.Close()
				return fmt.Errorf("decode %s document: %w", relationship.typeName, err)
			}
			otherDocumentIDs = append(otherDocumentIDs, otherID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate %s documents: %w", relationship.typeName, err)
		}
		rows.Close()

		for _, otherDocumentID := range otherDocumentIDs {
			if _, err := createRelationshipTx(
				ctx,
				tx,
				documentID,
				otherDocumentID,
				relationship.typeName,
			); err != nil {
				return err
			}
		}
	}
	return nil
}

func createRelationshipTx(
	ctx context.Context,
	tx *sql.Tx,
	documentA uuid.UUID,
	documentB uuid.UUID,
	relationshipType string,
) (uuid.UUID, error) {
	if string(documentA[:]) > string(documentB[:]) {
		documentA, documentB = documentB, documentA
	}

	var relationshipIDBytes []byte
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM document_relationships
		WHERE document_id_a = ? AND document_id_b = ? AND relationship_type = ?
	`, uuidToBytes(documentA), uuidToBytes(documentB), relationshipType).Scan(&relationshipIDBytes)
	if err == nil {
		return bytesToUUID(relationshipIDBytes)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("check relationship: %w", err)
	}

	relationshipID := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO document_relationships (
			id, document_id_a, document_id_b, relationship_type, confidence, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		uuidToBytes(relationshipID),
		uuidToBytes(documentA),
		uuidToBytes(documentB),
		relationshipType,
		nil,
		now,
		now,
	); err != nil {
		return uuid.Nil, fmt.Errorf("create relationship: %w", err)
	}
	return relationshipID, nil
}

func intelligenceRelationshipType(entityType document.EntityType) (string, bool) {
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
