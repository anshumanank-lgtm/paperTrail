package document

import (
	"time"

	"github.com/google/uuid"
)

// Document is the normalized representation of a converted file.
type Document struct {
	ID               uuid.UUID        `json:"id"`
	FileMetadata     FileMetadata     `json:"file_metadata"`
	DocumentMetadata DocumentMetadata `json:"document_metadata"`
}

// FileMetadata contains metadata about the source file itself.
type FileMetadata struct {
	SourcePath string    `json:"source_path"`
	Filename   string    `json:"filename"`
	Extension  string    `json:"extension"`
	Size       int64     `json:"size"`
	ModifiedAt time.Time `json:"modified_at"`
	Author     string    `json:"author,omitempty"`
	Creator    string    `json:"creator,omitempty"`
}

// DocumentMetadata contains semantic information extracted from the document.
type DocumentMetadata struct {
	Title        string           `json:"title,omitempty"`
	DocumentType DocumentType     `json:"document_type"`
	Entities     []Entity         `json:"entities,omitempty"`
	TypeData     DocumentTypeData `json:"type_data,omitempty"`
}

// DocumentType represents the standardized semantic type of a document.
type DocumentType string

const (
	DocumentTypeUnknown         DocumentType = "unknown"
	DocumentTypeInvoice         DocumentType = "invoice"
	DocumentTypeReceipt         DocumentType = "receipt"
	DocumentTypeBill            DocumentType = "bill"
	DocumentTypeStatement       DocumentType = "statement"
	DocumentTypeContract        DocumentType = "contract"
	DocumentTypeInsurancePolicy DocumentType = "insurance_policy"
	DocumentTypeTicket          DocumentType = "ticket"
	DocumentTypeIdentity        DocumentType = "identity"
	DocumentTypePayslip         DocumentType = "payslip"
	DocumentTypeTax             DocumentType = "tax"
	DocumentTypeWarranty        DocumentType = "warranty"
	DocumentTypeOther           DocumentType = "other"
)

// DocumentTypeData contains metadata specific to a document type.
type DocumentTypeData interface {
	DocumentType() DocumentType
}

// UnknownDocumentMetadata contains generic information when the document
// type cannot be determined.
type UnknownDocumentMetadata struct {
	Topics   []string `json:"topics,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
}

// OtherDocumentMetadata contains generic information for recognized documents
// that do not fit a supported document type.
type OtherDocumentMetadata struct {
	Topics   []string `json:"topics,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
}

// EntityType represents the standardized type of an extracted entity.
type EntityType string

const (
	// Tier 1 — Always extract
	EntityTypePerson       EntityType = "person"
	EntityTypeOrganisation EntityType = "organisation"
	EntityTypeLocation     EntityType = "location"
	EntityTypeDate         EntityType = "date"
	EntityTypeTime         EntityType = "time"
	EntityTypeMoney        EntityType = "money"
	EntityTypePercent      EntityType = "percent"
	EntityTypeQuantity     EntityType = "quantity"
	EntityTypeEmail        EntityType = "email"
	EntityTypePhoneNumber  EntityType = "phone_number"
	EntityTypeURL          EntityType = "url"
	EntityTypeProduct      EntityType = "product"
	EntityTypeEvent        EntityType = "event"
	EntityTypeVehicle      EntityType = "vehicle"

	// Tier 2 — Extract if present
	EntityTypeAddress      EntityType = "address"
	EntityTypeIDNumber     EntityType = "id_number"
	EntityTypeJobTitle     EntityType = "job_title"
	EntityTypeLaw          EntityType = "law"
	EntityTypeRegulation   EntityType = "regulation"
	EntityTypeLanguage     EntityType = "language"
	EntityTypeSocialHandle EntityType = "social_handle"
)

// Entity represents a named entity extracted from a document.
type Entity struct {
	ID    uuid.UUID
	Type  EntityType
	Value string
}
