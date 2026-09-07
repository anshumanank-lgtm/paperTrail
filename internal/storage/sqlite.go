package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"

	_ "modernc.org/sqlite"
)

type SQLiteStorage struct {
	db *sql.DB
	mu sync.Mutex
}

func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}

	_, err = db.Exec(`
    PRAGMA journal_mode = WAL;
    PRAGMA busy_timeout = 5000;
    PRAGMA synchronous = NORMAL;
	`)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("configure sqlite: %w", err)
	}

	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize SQLite schema: %w", err)
	}

	return &SQLiteStorage{
		db: db,
	}, nil
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

func uuidToBytes(id uuid.UUID) []byte {
	b := make([]byte, 16)
	copy(b, id[:])
	return b
}

func bytesToUUID(value []byte) (uuid.UUID, error) {
	if len(value) != 16 {
		return uuid.Nil, fmt.Errorf("invalid UUID length: %d", len(value))
	}

	var id uuid.UUID
	copy(id[:], value)

	return id, nil
}

func (s *SQLiteStorage) DumpTables(ctx context.Context) error {
	tables := []string{
		"documents",
		"entities",
		"document_entities",
		"document_relationships",
		"relationship_entities",
	}

	for _, table := range tables {
		fmt.Printf("\n========== %s ==========\n", table)

		rows, err := s.db.QueryContext(
			ctx,
			"SELECT * FROM "+table,
		)
		if err != nil {
			return fmt.Errorf("query %s: %w", table, err)
		}

		columns, err := rows.Columns()
		if err != nil {
			rows.Close()
			return fmt.Errorf("get columns for %s: %w", table, err)
		}

		fmt.Println(strings.Join(columns, " | "))

		for rows.Next() {
			values := make([]any, len(columns))
			valuePointers := make([]any, len(columns))

			for i := range values {
				valuePointers[i] = &values[i]
			}

			if err := rows.Scan(valuePointers...); err != nil {
				rows.Close()
				return fmt.Errorf("scan %s: %w", table, err)
			}

			for i, value := range values {
				if i > 0 {
					fmt.Print(" | ")
				}

				if bytes, ok := value.([]byte); ok && len(bytes) == 16 {
					if id, err := bytesToUUID(bytes); err == nil {
						fmt.Print(id)
						continue
					}
				}

				fmt.Print(value)
			}

			fmt.Println()
		}

		if err := rows.Err(); err != nil {
			rows.Close()
			return fmt.Errorf("iterate %s: %w", table, err)
		}

		rows.Close()
	}

	return nil
}
