package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"papertrail/internal/document"
)

func (s *SQLiteStorage) CreateDocument(
	ctx context.Context,
	doc document.Document,
	fingerprint string,
) (uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	id := doc.ID
	if id == uuid.Nil {
		id = uuid.New()
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO documents (
			id,
			source_path,
			filename,
			extension,
			size,
			modified_at,
			fingerprint,
			title,
			document_type,
			author,
			creator,
			created_at,
			updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		uuidToBytes(id),
		doc.FileMetadata.SourcePath,
		doc.FileMetadata.Filename,
		doc.FileMetadata.Extension,
		doc.FileMetadata.Size,
		doc.FileMetadata.ModifiedAt.UTC().Format(time.RFC3339Nano),
		fingerprint,
		doc.DocumentMetadata.Title,
		doc.DocumentMetadata.DocumentType,
		doc.FileMetadata.Author,
		doc.FileMetadata.Creator,
		now,
		now,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("create document: %w", err)
	}

	return id, nil
}

func (s *SQLiteStorage) GetDocument(
	ctx context.Context,
	id uuid.UUID,
) (*document.Document, error) {
	row := s.db.QueryRowContext(ctx, `
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
		WHERE id = ?
	`, uuidToBytes(id))

	doc, err := scanDocument(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("document not found: %s", id)
		}

		return nil, fmt.Errorf("get document: %w", err)
	}

	return doc, nil
}

func (s *SQLiteStorage) GetDocumentByPath(
	ctx context.Context,
	sourcePath string,
) (*document.Document, error) {
	row := s.db.QueryRowContext(ctx, `
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
		WHERE source_path = ?
	`, sourcePath)

	doc, err := scanDocument(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("document not found: %s", sourcePath)
		}

		return nil, fmt.Errorf("get document by path: %w", err)
	}

	return doc, nil
}

func (s *SQLiteStorage) UpdateDocument(
	ctx context.Context,
	id uuid.UUID,
	doc document.Document,
	fingerprint string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	result, err := s.db.ExecContext(ctx, `
		UPDATE documents
		SET
			source_path = ?,
			filename = ?,
			extension = ?,
			size = ?,
			modified_at = ?,
			fingerprint = ?,
			title = ?,
			document_type = ?,
			author = ?,
			creator = ?,
			updated_at = ?
		WHERE id = ?
	`,
		doc.FileMetadata.SourcePath,
		doc.FileMetadata.Filename,
		doc.FileMetadata.Extension,
		doc.FileMetadata.Size,
		doc.FileMetadata.ModifiedAt.UTC().Format(time.RFC3339Nano),
		fingerprint,
		doc.DocumentMetadata.Title,
		doc.DocumentMetadata.DocumentType,
		doc.FileMetadata.Author,
		doc.FileMetadata.Creator,
		now,
		uuidToBytes(id),
	)
	if err != nil {
		return fmt.Errorf("update document: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check document update: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("document not found: %s", id)
	}

	return nil
}

func (s *SQLiteStorage) DeleteDocument(
	ctx context.Context,
	id uuid.UUID,
) error {
	result, err := s.db.ExecContext(ctx, `
		DELETE FROM documents
		WHERE id = ?
	`, uuidToBytes(id))
	if err != nil {
		return fmt.Errorf("delete document: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check document deletion: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("document not found: %s", id)
	}

	return nil
}

func (s *SQLiteStorage) ListDocuments(
	ctx context.Context,
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
		ORDER BY modified_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}
	defer rows.Close()

	var documents []document.Document

	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			return nil, fmt.Errorf("scan document: %w", err)
		}

		documents = append(documents, *doc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate documents: %w", err)
	}

	return documents, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDocument(row rowScanner) (*document.Document, error) {
	var (
		idBytes      []byte
		sourcePath   string
		filename     string
		extension    string
		size         int64
		modifiedAt   string
		title        sql.NullString
		documentType string
		author       sql.NullString
		creator      sql.NullString
	)

	if err := row.Scan(
		&idBytes,
		&sourcePath,
		&filename,
		&extension,
		&size,
		&modifiedAt,
		&title,
		&documentType,
		&author,
		&creator,
	); err != nil {
		return nil, err
	}

	id, err := bytesToUUID(idBytes)
	if err != nil {
		return nil, fmt.Errorf("decode document id: %w", err)
	}

	parsedModifiedAt, err := time.Parse(time.RFC3339Nano, modifiedAt)
	if err != nil {
		return nil, fmt.Errorf("parse modified_at: %w", err)
	}

	return &document.Document{
		ID: id,
		FileMetadata: document.FileMetadata{
			SourcePath: sourcePath,
			Filename:   filename,
			Extension:  extension,
			Size:       size,
			ModifiedAt: parsedModifiedAt,
			Author:     author.String,
			Creator:    creator.String,
		},
		DocumentMetadata: document.DocumentMetadata{
			Title:        title.String,
			DocumentType: document.DocumentType(documentType),
		},
	}, nil
}

func (s *SQLiteStorage) GetDocumentsByAuthor(
	ctx context.Context,
	author string,
) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM documents
		WHERE lower(trim(author)) = lower(trim(?))
	`, author)
	if err != nil {
		return nil, fmt.Errorf("get documents by author: %w", err)
	}
	defer rows.Close()

	var documentIDs []uuid.UUID

	for rows.Next() {
		var idBytes []byte

		if err := rows.Scan(&idBytes); err != nil {
			return nil, fmt.Errorf("scan document id: %w", err)
		}

		id, err := bytesToUUID(idBytes)
		if err != nil {
			return nil, fmt.Errorf("decode document id: %w", err)
		}

		documentIDs = append(documentIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate author document ids: %w", err)
	}

	return documentIDs, nil
}

func (s *SQLiteStorage) GetDocumentsByCreator(
	ctx context.Context,
	creator string,
) ([]uuid.UUID, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id
		FROM documents
		WHERE lower(trim(creator)) = lower(trim(?))
	`, creator)
	if err != nil {
		return nil, fmt.Errorf("get documents by creator: %w", err)
	}
	defer rows.Close()

	var documentIDs []uuid.UUID

	for rows.Next() {
		var idBytes []byte

		if err := rows.Scan(&idBytes); err != nil {
			return nil, fmt.Errorf("scan document id: %w", err)
		}

		id, err := bytesToUUID(idBytes)
		if err != nil {
			return nil, fmt.Errorf("decode document id: %w", err)
		}

		documentIDs = append(documentIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate creator document ids: %w", err)
	}

	return documentIDs, nil
}
