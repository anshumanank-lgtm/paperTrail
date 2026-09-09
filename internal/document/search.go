package document

import "github.com/google/uuid"

type DocumentTextMatch struct {
	Document Document
	Entities []Entity
}

type DocumentRelationship struct {
	ID               uuid.UUID
	DocumentA        uuid.UUID
	DocumentB        uuid.UUID
	RelationshipType string
	Confidence       *float64
	Entities         []Entity
}
