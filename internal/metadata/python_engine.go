package metadata

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/google/uuid"

	"papertrail/internal/document"
)

const (
	titleConfidenceThreshold        = 0.50
	documentTypeConfidenceThreshold = 0.70
	entityConfidenceThreshold       = 0.80
)

type PythonMetadataEngine struct {
	cmd    *exec.Cmd
	stdin  *json.Encoder
	stdout *bufio.Scanner
	mu     sync.Mutex
}

type intelligenceRequest struct {
	Text string `json:"text"`
}

type intelligenceResponse struct {
	Title                  string               `json:"title"`
	TitleConfidence        float64              `json:"title_confidence"`
	DocumentType           string               `json:"document_type"`
	DocumentTypeConfidence float64              `json:"document_type_confidence"`
	Entities               []intelligenceEntity `json:"entities"`
}

type intelligenceEntity struct {
	Type       string  `json:"type"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
}

func NewPythonMetadataEngine() MetadataEngine {
	cmd := exec.Command(
		".venv/bin/python",
		"internal/intelligence/engine.py",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		panic(fmt.Sprintf("create Python stdin: %v", err))
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(fmt.Sprintf("create Python stdout: %v", err))
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		panic(fmt.Sprintf("start Python intelligence engine: %v", err))
	}

	return &PythonMetadataEngine{
		cmd:    cmd,
		stdin:  json.NewEncoder(stdin),
		stdout: bufio.NewScanner(stdout),
	}
}

func (e *PythonMetadataEngine) Extract(
	content document.ExtractedContent,
) document.DocumentMetadata {
	e.mu.Lock()
	defer e.mu.Unlock()

	request := intelligenceRequest{
		Text: content.Text,
	}

	if err := e.stdin.Encode(request); err != nil {
		panic(fmt.Sprintf("send request to Python intelligence engine: %v", err))
	}

	if !e.stdout.Scan() {
		panic("Python intelligence engine returned no response")
	}

	var response intelligenceResponse

	if err := json.Unmarshal(e.stdout.Bytes(), &response); err != nil {
		panic(fmt.Sprintf("decode Python intelligence response: %v", err))
	}

	return mapMetadata(response)
}

func mapMetadata(response intelligenceResponse) document.DocumentMetadata {
	title := ""

	if response.TitleConfidence >= titleConfidenceThreshold {
		title = response.Title
	}

	documentType := document.DocumentTypeUnknown

	if response.DocumentTypeConfidence >= documentTypeConfidenceThreshold {
		documentType = mapDocumentType(response.DocumentType)
	}

	metadata := document.DocumentMetadata{
		Title:        title,
		DocumentType: documentType,
	}

	for _, entity := range response.Entities {
		if entity.Confidence < entityConfidenceThreshold {
			continue
		}

		entityType, ok := mapEntityType(entity.Type)
		if !ok {
			continue
		}

		metadata.Entities = append(
			metadata.Entities,
			document.Entity{
				ID:    uuid.New(),
				Type:  entityType,
				Value: entity.Value,
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
		return document.DocumentTypeUnknown
	}
}

func mapEntityType(value string) (document.EntityType, bool) {
	switch value {
	case "person":
		return document.EntityTypePerson, true
	case "organisation":
		return document.EntityTypeOrganisation, true
	case "location":
		return document.EntityTypeLocation, true
	case "date":
		return document.EntityTypeDate, true
	case "time":
		return document.EntityTypeTime, true
	case "money":
		return document.EntityTypeMoney, true
	case "percent":
		return document.EntityTypePercent, true
	case "quantity":
		return document.EntityTypeQuantity, true
	case "email":
		return document.EntityTypeEmail, true
	case "phone_number":
		return document.EntityTypePhoneNumber, true
	case "url":
		return document.EntityTypeURL, true
	case "product":
		return document.EntityTypeProduct, true
	case "event":
		return document.EntityTypeEvent, true
	case "address":
		return document.EntityTypeAddress, true
	case "id_number":
		return document.EntityTypeIDNumber, true
	case "job_title":
		return document.EntityTypeJobTitle, true
	case "law":
		return document.EntityTypeLaw, true
	case "regulation":
		return document.EntityTypeRegulation, true
	case "language":
		return document.EntityTypeLanguage, true
	case "social_handle":
		return document.EntityTypeSocialHandle, true
	case "vehicle":
		return document.EntityTypeVehicle, true
	default:
		return "", false
	}
}
