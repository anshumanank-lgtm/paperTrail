package search

import (
	"github.com/google/uuid"

	"papertrail/internal/document"
	"papertrail/internal/storage"
)

type SearchResult struct {
	Document        document.Document
	MatchedEntities []document.Entity
}

type RelationshipDetail struct {
	Relationship  storage.DocumentRelationship
	OtherDocument document.Document
}

type DocumentDetail struct {
	Document      document.Document
	Entities      []document.Entity
	Relationships []RelationshipDetail
}

type EntityDetail struct {
	Entity    document.Entity
	Documents []document.Document
}

type SearchTextMatch struct {
	Document document.Document
	Entities []document.Entity
}

type Relationship struct {
	ID               uuid.UUID
	DocumentA        uuid.UUID
	DocumentB        uuid.UUID
	RelationshipType string
	Confidence       *float64
	Entities         []document.Entity
}
