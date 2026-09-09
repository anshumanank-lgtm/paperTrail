package search

import (
	"papertrail/internal/document"
	"papertrail/internal/storage"
)

type SearchResult struct {
	Document        document.Document
	MatchedEntities []document.Entity
}

type DocumentDetail struct {
	Document      document.Document
	Entities      []document.Entity
	Relationships []storage.DocumentRelationship
}

type SearchTextMatch struct {
	Document document.Document
	Entities []document.Entity
}
