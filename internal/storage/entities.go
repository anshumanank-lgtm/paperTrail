package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"papertrail/internal/document"
)

func (s *SQLiteStorage) GetOrCreateEntity(
	ctx context.Context,
	entity document.Entity,
) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	normalizedValue := normalizeEntityValue(entity.Value)
	now := time.Now().UTC().Format(time.RFC3339Nano)

	var idBytes []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM entities
		WHERE type = ?
		  AND normalized_value = ?
	`,
		entity.Type,
		normalizedValue,
	).Scan(&idBytes)

	if err == nil {
		id, err := bytesToUUID(idBytes)
		if err != nil {
			return uuid.Nil, fmt.Errorf("decode entity id: %w", err)
		}

		return id, nil
	}

	if err != sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("find entity: %w", err)
	}

	id := entity.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO entities (
			id,
			type,
			value,
			normalized_value,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		uuidToBytes(id),
		entity.Type,
		entity.Value,
		normalizedValue,
		now,
		now,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create entity: %w", err)
	}

	return id, nil
}

func (s *SQLiteStorage) AttachEntityToDocument(
	ctx context.Context,
	documentID uuid.UUID,
	entityID uuid.UUID,
	role string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO document_entities (
			document_id,
			entity_id,
			role
		)
		VALUES (?, ?, ?)
	`,
		uuidToBytes(documentID),
		uuidToBytes(entityID),
		role,
	)
	if err != nil {
		return fmt.Errorf("attach entity to document: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) ReplaceDocumentEntities(
	ctx context.Context,
	documentID uuid.UUID,
	entities []document.Entity,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace document entities transaction: %w", err)
	}
	defer tx.Rollback()

	oldEntityIDs, err := queryDocumentEntityIDs(ctx, tx, documentID)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM document_entities
		WHERE document_id = ?
	`, uuidToBytes(documentID)); err != nil {
		return fmt.Errorf("remove existing document entities: %w", err)
	}

	for i := range entities {
		entity := &entities[i]
		normalizedValue := normalizeEntityValue(entity.Value)
		if normalizedValue == "" {
			continue
		}

		entityID, err := getOrCreateEntityTx(ctx, tx, *entity, normalizedValue)
		if err != nil {
			return fmt.Errorf("store entity %q: %w", entity.Value, err)
		}
		entity.ID = entityID

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

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace document entities: %w", err)
	}
	return nil
}

func queryDocumentEntityIDs(ctx context.Context, tx *sql.Tx, documentID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT entity_id
		FROM document_entities
		WHERE document_id = ?
	`, uuidToBytes(documentID))
	if err != nil {
		return nil, fmt.Errorf("get existing document entity ids: %w", err)
	}
	defer rows.Close()

	var entityIDs []uuid.UUID
	for rows.Next() {
		var idBytes []byte
		if err := rows.Scan(&idBytes); err != nil {
			return nil, fmt.Errorf("scan existing entity id: %w", err)
		}
		id, err := bytesToUUID(idBytes)
		if err != nil {
			return nil, fmt.Errorf("decode existing entity id: %w", err)
		}
		entityIDs = append(entityIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate existing entity ids: %w", err)
	}
	return entityIDs, nil
}

func getOrCreateEntityTx(
	ctx context.Context,
	tx *sql.Tx,
	entity document.Entity,
	normalizedValue string,
) (uuid.UUID, error) {
	var idBytes []byte
	err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM entities
		WHERE type = ?
		  AND normalized_value = ?
	`, entity.Type, normalizedValue).Scan(&idBytes)
	if err == nil {
		return bytesToUUID(idBytes)
	}
	if err != sql.ErrNoRows {
		return uuid.Nil, fmt.Errorf("find entity: %w", err)
	}

	id := entity.ID
	if id == uuid.Nil {
		id = uuid.New()
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO entities (id, type, value, normalized_value, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, uuidToBytes(id), entity.Type, entity.Value, normalizedValue, now, now); err != nil {
		return uuid.Nil, fmt.Errorf("create entity: %w", err)
	}
	return id, nil
}

func (s *SQLiteStorage) GetDocumentEntities(
	ctx context.Context,
	documentID uuid.UUID,
) ([]document.Entity, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			e.id,
			e.type,
			e.value
		FROM entities e
		INNER JOIN document_entities de
			ON de.entity_id = e.id
		WHERE de.document_id = ?
		ORDER BY e.type, e.value
	`, uuidToBytes(documentID))
	if err != nil {
		return nil, fmt.Errorf("get document entities: %w", err)
	}
	defer rows.Close()

	var entities []document.Entity

	for rows.Next() {
		var (
			idBytes []byte
			entity  document.Entity
		)

		if err := rows.Scan(
			&idBytes,
			&entity.Type,
			&entity.Value,
		); err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}

		entity.ID, err = bytesToUUID(idBytes)
		if err != nil {
			return nil, fmt.Errorf("decode entity id: %w", err)
		}

		entities = append(entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entities: %w", err)
	}

	return entities, nil
}

func normalizeEntityValue(value string) string {
	return strings.Join(
		strings.Fields(
			strings.ToLower(
				strings.TrimSpace(value),
			),
		),
		" ",
	)
}

func (s *SQLiteStorage) GetDocumentsByEntity(
	ctx context.Context,
	entityID uuid.UUID,
) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT document_id
		FROM document_entities
		WHERE entity_id = ?
	`,
		uuidToBytes(entityID),
	)
	if err != nil {
		return nil, fmt.Errorf("get documents by entity: %w", err)
	}
	defer rows.Close()

	var documentIDs []uuid.UUID

	for rows.Next() {
		var idBytes []byte

		if err := rows.Scan(&idBytes); err != nil {
			return nil, fmt.Errorf("scan document id: %w", err)
		}

		documentID, err := bytesToUUID(idBytes)
		if err != nil {
			return nil, fmt.Errorf("decode document id: %w", err)
		}

		documentIDs = append(documentIDs, documentID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate document ids: %w", err)
	}

	return documentIDs, nil
}

func (s *SQLiteStorage) GetDocumentEntityIDs(
	ctx context.Context,
	documentID uuid.UUID,
) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT entity_id
		FROM document_entities
		WHERE document_id = ?
	`,
		uuidToBytes(documentID),
	)
	if err != nil {
		return nil, fmt.Errorf("get document entity ids: %w", err)
	}
	defer rows.Close()

	var entityIDs []uuid.UUID

	for rows.Next() {
		var idBytes []byte

		if err := rows.Scan(&idBytes); err != nil {
			return nil, fmt.Errorf("scan entity id: %w", err)
		}

		entityID, err := bytesToUUID(idBytes)
		if err != nil {
			return nil, fmt.Errorf("decode entity id: %w", err)
		}

		entityIDs = append(entityIDs, entityID)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate entity ids: %w", err)
	}

	return entityIDs, nil
}
