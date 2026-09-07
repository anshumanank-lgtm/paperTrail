package metadata

import (
	"context"

	"github.com/google/uuid"

	"papertrail/internal/document"
	intelligencepb "papertrail/internal/intelligence/proto"
)

const (
	titleConfidenceThreshold        = 0.50
	documentTypeConfidenceThreshold = 0.70
	entityConfidenceThreshold       = 0.80
)

type GRPCMetadataEngine struct {
	client intelligencepb.IntelligenceServiceClient
}

func NewGRPCMetadataEngine(
	client intelligencepb.IntelligenceServiceClient,
) MetadataEngine {
	return &GRPCMetadataEngine{
		client: client,
	}
}

func (e *GRPCMetadataEngine) Extract(
	content document.ExtractedContent,
) document.DocumentMetadata {

	response, err := e.client.Extract(
		context.Background(),
		&intelligencepb.ExtractRequest{
			Text: content.Text,
		},
	)
	if err != nil {
		panic(err)
	}

	return mapMetadata(response)
}

func mapMetadata(
	response *intelligencepb.ExtractResponse,
) document.DocumentMetadata {

	metadata := document.DocumentMetadata{}

	if response.GetTitleConfidence() >= titleConfidenceThreshold {
		metadata.Title = response.GetTitle()
	}

	if response.GetDocumentTypeConfidence() >= documentTypeConfidenceThreshold {
		metadata.DocumentType = mapDocumentType(response.GetDocumentType())
	} else {
		metadata.DocumentType = document.DocumentTypeOther
	}

	for _, entity := range response.GetEntities() {
		if entity.GetConfidence() < entityConfidenceThreshold {
			continue
		}

		metadata.Entities = append(
			metadata.Entities,
			document.Entity{
				ID:    uuid.New(),
				Type:  mapEntityType(entity.GetType()),
				Value: entity.GetValue(),
			},
		)
	}

	return metadata
}

func mapDocumentType(value string) document.DocumentType {
	switch value {
	case "invoice":
		return document.DocumentTypeInvoice
	case "receipt":
		return document.DocumentTypeReceipt
	case "bill":
		return document.DocumentTypeBill
	case "statement":
		return document.DocumentTypeStatement
	case "contract":
		return document.DocumentTypeContract
	case "insurance_policy":
		return document.DocumentTypeInsurancePolicy
	case "ticket":
		return document.DocumentTypeTicket
	case "identity":
		return document.DocumentTypeIdentity
	case "payslip":
		return document.DocumentTypePayslip
	case "tax":
		return document.DocumentTypeTax
	case "warranty":
		return document.DocumentTypeWarranty
	case "other":
		return document.DocumentTypeOther
	default:
		return document.DocumentTypeOther
	}
}

func mapEntityType(value string) document.EntityType {
	switch value {
	case "person":
		return document.EntityTypePerson
	case "organisation":
		return document.EntityTypeOrganisation
	case "location":
		return document.EntityTypeLocation
	case "date":
		return document.EntityTypeDate
	case "time":
		return document.EntityTypeTime
	case "money":
		return document.EntityTypeMoney
	case "percent":
		return document.EntityTypePercent
	case "quantity":
		return document.EntityTypeQuantity
	case "email":
		return document.EntityTypeEmail
	case "phone_number":
		return document.EntityTypePhoneNumber
	case "url":
		return document.EntityTypeURL
	case "product":
		return document.EntityTypeProduct
	case "event":
		return document.EntityTypeEvent
	case "vehicle":
		return document.EntityTypeVehicle
	case "address":
		return document.EntityTypeAddress
	case "id_number":
		return document.EntityTypeIDNumber
	case "job_title":
		return document.EntityTypeJobTitle
	case "law":
		return document.EntityTypeLaw
	case "regulation":
		return document.EntityTypeRegulation
	case "language":
		return document.EntityTypeLanguage
	case "social_handle":
		return document.EntityTypeSocialHandle
	default:
		return document.EntityType("")
	}
}
