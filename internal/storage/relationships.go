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

func (s *SQLiteStorage) CreateRelationship(
	ctx context.Context,
	documentA uuid.UUID,
	documentB uuid.UUID,
	relationshipType string,
	confidence *float64,
) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if documentA == uuid.Nil || documentB == uuid.Nil {
		return uuid.Nil, fmt.Errorf("document IDs cannot be nil")
	}

	if documentA == documentB {
		return uuid.Nil, fmt.Errorf("cannot create relationship between the same document")
	}

	// Canonical ordering so A/B always represent the same pair.
	if string(documentA[:]) > string(documentB[:]) {
		documentA, documentB = documentB, documentA
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)

	var relationshipIDBytes []byte

	// Check whether the relationship already exists.
	err := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM document_relationships
		WHERE document_id_a = ?
		  AND document_id_b = ?
		  AND relationship_type = ?
	`,
		uuidToBytes(documentA),
		uuidToBytes(documentB),
		relationshipType,
	).Scan(&relationshipIDBytes)

	if err == nil {
		relationshipID, decodeErr := bytesToUUID(relationshipIDBytes)
		if decodeErr != nil {
			return uuid.Nil, fmt.Errorf("decode relationship id: %w", decodeErr)
		}
		_, err = s.db.ExecContext(ctx, `
			UPDATE document_relationships
			SET confidence = ?,
			    updated_at = ?
			WHERE id = ?
		`,
			confidence,
			now,
			uuidToBytes(relationshipID),
		)
		if err != nil {
			return uuid.Nil, fmt.Errorf("update relationship: %w", err)
		}

		return relationshipID, nil
	}

	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("check relationship: %w", err)
	}

	relationshipID := uuid.New()

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO document_relationships (
			id,
			document_id_a,
			document_id_b,
			relationship_type,
			confidence,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		uuidToBytes(relationshipID),
		uuidToBytes(documentA),
		uuidToBytes(documentB),
		relationshipType,
		confidence,
		now,
		now,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create relationship: %w", err)
	}

	return relationshipID, nil
}

func (s *SQLiteStorage) AttachEntityToRelationship(
	ctx context.Context,
	relationshipID uuid.UUID,
	entityID uuid.UUID,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO relationship_entities (
			relationship_id,
			entity_id
		)
		VALUES (?, ?)
	`,
		uuidToBytes(relationshipID),
		uuidToBytes(entityID),
	)
	if err != nil {
		return fmt.Errorf("attach entity to relationship: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) DeleteDocumentRelationships(
	ctx context.Context,
	documentID uuid.UUID,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.ExecContext(ctx, `
		DELETE FROM document_relationships
		WHERE document_id_a = ?
		   OR document_id_b = ?
	`, uuidToBytes(documentID), uuidToBytes(documentID))
	if err != nil {
		return fmt.Errorf("delete document relationships: %w", err)
	}
	return nil
}

func (s *SQLiteStorage) GetDocumentRelationships(
	ctx context.Context,
	documentID uuid.UUID,
) ([]DocumentRelationship, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			dr.id,
			dr.document_id_a,
			dr.document_id_b,
			dr.relationship_type,
			dr.confidence
		FROM document_relationships dr
		WHERE dr.document_id_a = ?
		   OR dr.document_id_b = ?
		ORDER BY dr.updated_at DESC
	`,
		uuidToBytes(documentID),
		uuidToBytes(documentID),
	)
	if err != nil {
		return nil, fmt.Errorf("get document relationships: %w", err)
	}

	var relationships []DocumentRelationship

	for rows.Next() {
		var (
			relationshipID []byte
			documentA      []byte
			documentB      []byte
			relationship   DocumentRelationship
		)

		if err := rows.Scan(
			&relationshipID,
			&documentA,
			&documentB,
			&relationship.RelationshipType,
			&relationship.Confidence,
		); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan relationship: %w", err)
		}

		relationship.ID, err = bytesToUUID(relationshipID)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode relationship ID: %w", err)
		}

		relationship.DocumentA, err = bytesToUUID(documentA)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode document A ID: %w", err)
		}

		relationship.DocumentB, err = bytesToUUID(documentB)
		if err != nil {
			rows.Close()
			return nil, fmt.Errorf("decode document B ID: %w", err)
		}

		relationships = append(relationships, relationship)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("iterate relationships: %w", err)
	}

	// Important: release the SQLite connection before issuing
	// additional queries for relationship entities.
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("close relationship rows: %w", err)
	}

	for i := range relationships {
		entities, err := s.getRelationshipEntities(
			ctx,
			relationships[i].ID,
		)
		if err != nil {
			return nil, err
		}

		relationships[i].Entities = entities
	}

	return relationships, nil
}

func (s *SQLiteStorage) getRelationshipEntities(
	ctx context.Context,
	relationshipID uuid.UUID,
) ([]document.Entity, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			e.id,
			e.type,
			e.value
		FROM entities e
		INNER JOIN relationship_entities re
			ON re.entity_id = e.id
		WHERE re.relationship_id = ?
		ORDER BY e.type, e.value
	`,
		uuidToBytes(relationshipID),
	)
	if err != nil {
		return nil, fmt.Errorf("get relationship entities: %w", err)
	}
	defer rows.Close()

	var entities []document.Entity

	for rows.Next() {
		var (
			entityID []byte
			entity   document.Entity
		)

		if err := rows.Scan(
			&entityID,
			&entity.Type,
			&entity.Value,
		); err != nil {
			return nil, fmt.Errorf("scan relationship entity: %w", err)
		}

		entity.ID, err = bytesToUUID(entityID)
		if err != nil {
			return nil, fmt.Errorf("decode relationship entity ID: %w", err)
		}

		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate relationship entities: %w", err)
	}

	return entities, nil
}
