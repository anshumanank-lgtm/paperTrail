package storage

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/google/uuid"
)

const embeddingDimensions = 384

var ErrStaleDocument = errors.New("stale document")

func (s *SQLiteStorage) CreateChunks(
	ctx context.Context,
	documentID uuid.UUID,
	chunks []Chunk,
) error {
	if len(chunks) == 0 {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create chunks transaction: %w", err)
	}
	defer tx.Rollback()

	for _, chunk := range chunks {
		if chunk.DocumentID != uuid.Nil && chunk.DocumentID != documentID {
			return fmt.Errorf(
				"chunk document ID mismatch: %s",
				chunk.DocumentID,
			)
		}

		if len(chunk.Embedding) != embeddingDimensions {
			return fmt.Errorf(
				"invalid embedding dimensions for chunk %d: got %d, want %d",
				chunk.ChunkIndex,
				len(chunk.Embedding),
				embeddingDimensions,
			)
		}

		if _, err := tx.ExecContext(ctx, `
			INSERT INTO chunks (
				document_id,
				chunk_index,
				text,
				embedding
			)
			VALUES (?, ?, ?, ?)
		`,
			uuidToBytes(documentID),
			chunk.ChunkIndex,
			chunk.Text,
			encodeEmbedding(chunk.Embedding),
		); err != nil {
			return fmt.Errorf(
				"create chunk %d: %w",
				chunk.ChunkIndex,
				err,
			)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create chunks: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) GetChunks(
	ctx context.Context,
	documentID uuid.UUID,
) ([]Chunk, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			document_id,
			chunk_index,
			text,
			embedding
		FROM chunks
		WHERE document_id = ?
		ORDER BY chunk_index
	`, uuidToBytes(documentID))
	if err != nil {
		return nil, fmt.Errorf("get chunks: %w", err)
	}
	defer rows.Close()

	var chunks []Chunk

	for rows.Next() {
		var (
			chunk          Chunk
			documentIDData []byte
			embeddingData  []byte
		)

		if err := rows.Scan(
			&chunk.ID,
			&documentIDData,
			&chunk.ChunkIndex,
			&chunk.Text,
			&embeddingData,
		); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}

		id, err := bytesToUUID(documentIDData)
		if err != nil {
			return nil, fmt.Errorf(
				"decode chunk document ID: %w",
				err,
			)
		}

		embedding, err := decodeEmbedding(embeddingData)
		if err != nil {
			return nil, fmt.Errorf(
				"decode chunk embedding: %w",
				err,
			)
		}

		chunk.DocumentID = id
		chunk.Embedding = embedding

		chunks = append(chunks, chunk)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chunks: %w", err)
	}

	return chunks, nil
}

func (s *SQLiteStorage) DeleteDocumentChunks(
	ctx context.Context,
	documentID uuid.UUID,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.ExecContext(ctx, `
		DELETE FROM chunks
		WHERE document_id = ?
	`, uuidToBytes(documentID)); err != nil {
		return fmt.Errorf("delete document chunks: %w", err)
	}

	return nil
}

func encodeEmbedding(values []float32) []byte {
	data := make([]byte, len(values)*4)

	for i, value := range values {
		bits := math.Float32bits(value)

		binary.LittleEndian.PutUint32(
			data[i*4:],
			bits,
		)
	}

	return data
}

func decodeEmbedding(data []byte) ([]float32, error) {
	if len(data)%4 != 0 {
		return nil, fmt.Errorf(
			"invalid embedding byte length: %d",
			len(data),
		)
	}

	values := make([]float32, len(data)/4)

	for i := range values {
		bits := binary.LittleEndian.Uint32(
			data[i*4:],
		)

		values[i] = math.Float32frombits(bits)
	}

	return values, nil
}

func (s *SQLiteStorage) ReplaceDocumentChunks(
	ctx context.Context,
	documentID uuid.UUID,
	fingerprint string,
	chunks []Chunk,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin replace chunks transaction: %w", err)
	}
	defer tx.Rollback()

	var currentFingerprint string

	err = tx.QueryRowContext(ctx, `
        SELECT fingerprint
        FROM documents
        WHERE id = ?
    `, uuidToBytes(documentID)).Scan(&currentFingerprint)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("document not found: %s", documentID)
		}
		return fmt.Errorf("get document fingerprint: %w", err)
	}

	if currentFingerprint != fingerprint {
		return ErrStaleDocument
	}

	if _, err := tx.ExecContext(ctx, `
        DELETE FROM chunks
        WHERE document_id = ?
    `, uuidToBytes(documentID)); err != nil {
		return fmt.Errorf("delete document chunks: %w", err)
	}

	for _, chunk := range chunks {
		if len(chunk.Embedding) != embeddingDimensions {
			return fmt.Errorf(
				"invalid embedding dimensions for chunk %d: got %d, want %d",
				chunk.ChunkIndex,
				len(chunk.Embedding),
				embeddingDimensions,
			)
		}

		if _, err := tx.ExecContext(ctx, `
            INSERT INTO chunks (
                document_id,
                chunk_index,
                text,
                embedding
            )
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

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit replace chunks: %w", err)
	}

	return nil
}

func (s *SQLiteStorage) SearchChunks(
	ctx context.Context,
	embedding []float32,
	limit int,
) ([]ChunkSearchResult, error) {
	if len(embedding) != embeddingDimensions {
		return nil, fmt.Errorf(
			"invalid query embedding dimensions: got %d, want %d",
			len(embedding),
			embeddingDimensions,
		)
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT
			id,
			document_id,
			chunk_index,
			text,
			embedding
		FROM chunks
	`)
	if err != nil {
		return nil, fmt.Errorf("search chunks: %w", err)
	}
	defer rows.Close()

	results := make([]ChunkSearchResult, 0)

	for rows.Next() {
		var (
			chunk          Chunk
			documentIDData []byte
			embeddingData  []byte
		)

		if err := rows.Scan(
			&chunk.ID,
			&documentIDData,
			&chunk.ChunkIndex,
			&chunk.Text,
			&embeddingData,
		); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}

		documentID, err := bytesToUUID(documentIDData)
		if err != nil {
			return nil, fmt.Errorf(
				"decode chunk document ID: %w",
				err,
			)
		}

		storedEmbedding, err := decodeEmbedding(embeddingData)
		if err != nil {
			return nil, fmt.Errorf(
				"decode chunk embedding: %w",
				err,
			)
		}

		chunk.DocumentID = documentID
		chunk.Embedding = storedEmbedding

		results = append(results, ChunkSearchResult{
			Chunk: chunk,
			Similarity: cosineSimilarity(
				embedding,
				storedEmbedding,
			),
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chunks: %w", err)
	}

	sort.Slice(
		results,
		func(i, j int) bool {
			return results[i].Similarity > results[j].Similarity
		},
	)

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func cosineSimilarity(
	a []float32,
	b []float32,
) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dot float32
	var normA float32
	var normB float32

	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / float32(
		math.Sqrt(float64(normA*normB)),
	)
}
