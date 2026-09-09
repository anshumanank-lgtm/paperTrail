package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"papertrail/internal/document"
)

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	value = strings.ReplaceAll(value, `_`, `\_`)
	return value
}

func (s *SQLiteStorage) SearchByFilename(
	ctx context.Context,
	value string,
) ([]document.Document, error) {
	return s.searchDocuments(ctx, `
		WHERE LOWER(filename) LIKE LOWER(?) ESCAPE '\'
		ORDER BY modified_at DESC
	`, "%"+escapeLike(value)+"%")
}

func (s *SQLiteStorage) SearchByAuthor(
	ctx context.Context,
	value string,
) ([]document.Document, error) {
	return s.searchDocuments(ctx, `
		WHERE LOWER(COALESCE(author, '')) LIKE LOWER(?) ESCAPE '\'
		ORDER BY modified_at DESC
	`, "%"+escapeLike(value)+"%")
}

func (s *SQLiteStorage) SearchByCreator(
	ctx context.Context,
	value string,
) ([]document.Document, error) {
	return s.searchDocuments(ctx, `
		WHERE LOWER(COALESCE(creator, '')) LIKE LOWER(?) ESCAPE '\'
		ORDER BY modified_at DESC
	`, "%"+escapeLike(value)+"%")
}

func (s *SQLiteStorage) SearchByDate(
	ctx context.Context,
	value string,
) ([]document.Document, error) {
	return s.searchDocuments(ctx, `
		WHERE substr(modified_at, 1, 10) = ?
		ORDER BY modified_at DESC
	`, value)
}

func (s *SQLiteStorage) SearchByText(
	ctx context.Context,
	value string,
) ([]document.DocumentTextMatch, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			d.id,
			d.source_path,
			d.filename,
			d.extension,
			d.size,
			d.modified_at,
			d.title,
			d.document_type,
			d.author,
			d.creator,
			e.id,
			e.type,
			e.value
		FROM documents d
		INNER JOIN document_entities de
			ON de.document_id = d.id
		INNER JOIN entities e
			ON e.id = de.entity_id
		WHERE LOWER(e.normalized_value) LIKE LOWER(?) ESCAPE '\'
		ORDER BY d.modified_at DESC
	`, "%"+escapeLike(value)+"%")
	if err != nil {
		return nil, fmt.Errorf("search text: %w", err)
	}
	defer rows.Close()

	results := make(map[uuid.UUID]*document.DocumentTextMatch)
	order := make([]uuid.UUID, 0)

	for rows.Next() {
		var (
			docID         []byte
			entityID      []byte
			modifiedAtRaw string
			doc           document.Document
			entity        document.Entity
		)

		if err := rows.Scan(
			&docID,
			&doc.FileMetadata.SourcePath,
			&doc.FileMetadata.Filename,
			&doc.FileMetadata.Extension,
			&doc.FileMetadata.Size,
			&modifiedAtRaw,
			&doc.DocumentMetadata.Title,
			&doc.DocumentMetadata.DocumentType,
			&doc.FileMetadata.Author,
			&doc.FileMetadata.Creator,
			&entityID,
			&entity.Type,
			&entity.Value,
		); err != nil {
			return nil, fmt.Errorf("scan text search result: %w", err)
		}

		doc.ID, err = bytesToUUID(docID)
		if err != nil {
			return nil, fmt.Errorf("decode document ID: %w", err)
		}

		entity.ID, err = bytesToUUID(entityID)
		if err != nil {
			return nil, fmt.Errorf("decode entity ID: %w", err)
		}

		if doc.FileMetadata.ModifiedAt, err = time.Parse(time.RFC3339Nano, modifiedAtRaw); err != nil {
			return nil, fmt.Errorf("parse modified time: %w", err)
		}

		result, exists := results[doc.ID]
		if !exists {
			result = &document.DocumentTextMatch{
				Document: doc,
			}
			results[doc.ID] = result
			order = append(order, doc.ID)
		}

		result.Entities = append(result.Entities, entity)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate text search results: %w", err)
	}

	output := make([]document.DocumentTextMatch, 0, len(order))
	for _, id := range order {
		output = append(output, *results[id])
	}

	return output, nil
}

func (s *SQLiteStorage) searchDocuments(
	ctx context.Context,
	condition string,
	args ...any,
) ([]document.Document, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			source_path,
			filename,
			extension,
			size,
			modified_at,
			title,
			document_type,
			author,
			creator
		FROM documents
	`+condition, args...)
	if err != nil {
		return nil, fmt.Errorf("search documents: %w", err)
	}
	defer rows.Close()

	var documents []document.Document

	for rows.Next() {
		var (
			id          []byte
			modifiedRaw string
			doc         document.Document
		)

		if err := rows.Scan(
			&id,
			&doc.FileMetadata.SourcePath,
			&doc.FileMetadata.Filename,
			&doc.FileMetadata.Extension,
			&doc.FileMetadata.Size,
			&modifiedRaw,
			&doc.DocumentMetadata.Title,
			&doc.DocumentMetadata.DocumentType,
			&doc.FileMetadata.Author,
			&doc.FileMetadata.Creator,
		); err != nil {
			return nil, fmt.Errorf("scan search document: %w", err)
		}

		doc.ID, err = bytesToUUID(id)
		if err != nil {
			return nil, fmt.Errorf("decode document ID: %w", err)
		}

		doc.FileMetadata.ModifiedAt, err = time.Parse(time.RFC3339Nano, modifiedRaw)
		if err != nil {
			return nil, fmt.Errorf("parse modified time: %w", err)
		}

		documents = append(documents, doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate search documents: %w", err)
	}

	return documents, nil
}

func (s *SQLiteStorage) SearchByType(
	ctx context.Context,
	value string,
) ([]document.Document, error) {
	return s.searchDocuments(ctx, `
		WHERE LOWER(document_type) LIKE LOWER(?) ESCAPE '\'
		ORDER BY modified_at DESC
	`, "%"+escapeLike(value)+"%")
}
