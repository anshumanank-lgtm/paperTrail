package document

import "time"

// Document is the normalized representation of a converted file.
type Document struct {
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

func (*UnknownDocumentMetadata) DocumentType() DocumentType {
	return DocumentTypeUnknown
}

// OtherDocumentMetadata contains generic information for recognized documents
// that do not fit a supported document type.
type OtherDocumentMetadata struct {
	Topics   []string `json:"topics,omitempty"`
	Keywords []string `json:"keywords,omitempty"`
}

func (*OtherDocumentMetadata) DocumentType() DocumentType {
	return DocumentTypeOther
}

// InvoiceMetadata contains metadata specific to an invoice.
type InvoiceMetadata struct {
	InvoiceNumber string    `json:"invoice_number,omitempty"`
	Vendor        string    `json:"vendor,omitempty"`
	Customer      string    `json:"customer,omitempty"`
	InvoiceDate   time.Time `json:"invoice_date,omitempty"`
	DueDate       time.Time `json:"due_date,omitempty"`
	Currency      string    `json:"currency,omitempty"`
	Subtotal      float64   `json:"subtotal,omitempty"`
	Tax           float64   `json:"tax,omitempty"`
	Discount      float64   `json:"discount,omitempty"`
	Total         float64   `json:"total,omitempty"`
	AmountPaid    float64   `json:"amount_paid,omitempty"`
	AmountDue     float64   `json:"amount_due,omitempty"`
}

func (*InvoiceMetadata) DocumentType() DocumentType {
	return DocumentTypeInvoice
}

// ReceiptMetadata contains metadata specific to a receipt.
type ReceiptMetadata struct {
	ReceiptNumber   string    `json:"receipt_number,omitempty"`
	Merchant        string    `json:"merchant,omitempty"`
	Customer        string    `json:"customer,omitempty"`
	TransactionDate time.Time `json:"transaction_date,omitempty"`
	Currency        string    `json:"currency,omitempty"`
	Subtotal        float64   `json:"subtotal,omitempty"`
	Tax             float64   `json:"tax,omitempty"`
	Discount        float64   `json:"discount,omitempty"`
	Total           float64   `json:"total,omitempty"`
	PaymentMethod   string    `json:"payment_method,omitempty"`
}

func (*ReceiptMetadata) DocumentType() DocumentType {
	return DocumentTypeReceipt
}

// BillMetadata contains metadata specific to a bill.
type BillMetadata struct {
	BillNumber  string    `json:"bill_number,omitempty"`
	Provider    string    `json:"provider,omitempty"`
	Customer    string    `json:"customer,omitempty"`
	BillingDate time.Time `json:"billing_date,omitempty"`
	DueDate     time.Time `json:"due_date,omitempty"`
	Currency    string    `json:"currency,omitempty"`
	AmountDue   float64   `json:"amount_due,omitempty"`
	AmountPaid  float64   `json:"amount_paid,omitempty"`
}

func (*BillMetadata) DocumentType() DocumentType {
	return DocumentTypeBill
}

// StatementMetadata contains metadata specific to a financial statement.
type StatementMetadata struct {
	Provider       string    `json:"provider,omitempty"`
	AccountNumber  string    `json:"account_number,omitempty"`
	StatementDate  time.Time `json:"statement_date,omitempty"`
	PeriodStart    time.Time `json:"period_start,omitempty"`
	PeriodEnd      time.Time `json:"period_end,omitempty"`
	Currency       string    `json:"currency,omitempty"`
	OpeningBalance float64   `json:"opening_balance,omitempty"`
	ClosingBalance float64   `json:"closing_balance,omitempty"`
	AmountDue      float64   `json:"amount_due,omitempty"`
	MinimumDue     float64   `json:"minimum_due,omitempty"`
}

func (*StatementMetadata) DocumentType() DocumentType {
	return DocumentTypeStatement
}

// ContractMetadata contains metadata specific to a contract.
type ContractMetadata struct {
	ContractNumber    string    `json:"contract_number,omitempty"`
	Parties           []string  `json:"parties,omitempty"`
	EffectiveDate     time.Time `json:"effective_date,omitempty"`
	ExpiryDate        time.Time `json:"expiry_date,omitempty"`
	TerminationDate   time.Time `json:"termination_date,omitempty"`
	GoverningLocation string    `json:"governing_location,omitempty"`
}

func (*ContractMetadata) DocumentType() DocumentType {
	return DocumentTypeContract
}

// InsurancePolicyMetadata contains metadata specific to an insurance policy.
type InsurancePolicyMetadata struct {
	PolicyNumber string    `json:"policy_number,omitempty"`
	Provider     string    `json:"provider,omitempty"`
	Insured      string    `json:"insured,omitempty"`
	PolicyType   string    `json:"policy_type,omitempty"`
	StartDate    time.Time `json:"start_date,omitempty"`
	ExpiryDate   time.Time `json:"expiry_date,omitempty"`
	RenewalDate  time.Time `json:"renewal_date,omitempty"`
	Currency     string    `json:"currency,omitempty"`
	Premium      float64   `json:"premium,omitempty"`
	Deductible   float64   `json:"deductible,omitempty"`
}

func (*InsurancePolicyMetadata) DocumentType() DocumentType {
	return DocumentTypeInsurancePolicy
}

// TicketMetadata contains metadata specific to a ticket or booking.
type TicketMetadata struct {
	TicketNumber  string    `json:"ticket_number,omitempty"`
	Provider      string    `json:"provider,omitempty"`
	Passenger     string    `json:"passenger,omitempty"`
	Event         string    `json:"event,omitempty"`
	Origin        string    `json:"origin,omitempty"`
	Destination   string    `json:"destination,omitempty"`
	DepartureDate time.Time `json:"departure_date,omitempty"`
	ArrivalDate   time.Time `json:"arrival_date,omitempty"`
	EventDate     time.Time `json:"event_date,omitempty"`
}

func (*TicketMetadata) DocumentType() DocumentType {
	return DocumentTypeTicket
}

// IdentityMetadata contains metadata specific to an identity document.
type IdentityMetadata struct {
	DocumentNumber   string    `json:"document_number,omitempty"`
	Holder           string    `json:"holder,omitempty"`
	IssuingAuthority string    `json:"issuing_authority,omitempty"`
	IssueDate        time.Time `json:"issue_date,omitempty"`
	ExpiryDate       time.Time `json:"expiry_date,omitempty"`
	Nationality      string    `json:"nationality,omitempty"`
}

func (*IdentityMetadata) DocumentType() DocumentType {
	return DocumentTypeIdentity
}

// PayslipMetadata contains metadata specific to a payslip.
type PayslipMetadata struct {
	Employer       string    `json:"employer,omitempty"`
	Employee       string    `json:"employee,omitempty"`
	EmployeeID     string    `json:"employee_id,omitempty"`
	PayPeriodStart time.Time `json:"pay_period_start,omitempty"`
	PayPeriodEnd   time.Time `json:"pay_period_end,omitempty"`
	Currency       string    `json:"currency,omitempty"`
	GrossPay       float64   `json:"gross_pay,omitempty"`
	NetPay         float64   `json:"net_pay,omitempty"`
	Tax            float64   `json:"tax,omitempty"`
}

func (*PayslipMetadata) DocumentType() DocumentType {
	return DocumentTypePayslip
}

// TaxMetadata contains metadata specific to a tax document.
type TaxMetadata struct {
	Taxpayer       string    `json:"taxpayer,omitempty"`
	TaxAuthority   string    `json:"tax_authority,omitempty"`
	TaxYear        string    `json:"tax_year,omitempty"`
	DocumentNumber string    `json:"document_number,omitempty"`
	FilingDate     time.Time `json:"filing_date,omitempty"`
	DueDate        time.Time `json:"due_date,omitempty"`
	Currency       string    `json:"currency,omitempty"`
	TaxAmount      float64   `json:"tax_amount,omitempty"`
	AmountPaid     float64   `json:"amount_paid,omitempty"`
	AmountDue      float64   `json:"amount_due,omitempty"`
}

func (*TaxMetadata) DocumentType() DocumentType {
	return DocumentTypeTax
}

// WarrantyMetadata contains metadata specific to a warranty.
type WarrantyMetadata struct {
	Provider       string    `json:"provider,omitempty"`
	Product        string    `json:"product,omitempty"`
	SerialNumber   string    `json:"serial_number,omitempty"`
	PurchaseDate   time.Time `json:"purchase_date,omitempty"`
	StartDate      time.Time `json:"start_date,omitempty"`
	ExpiryDate     time.Time `json:"expiry_date,omitempty"`
	WarrantyNumber string    `json:"warranty_number,omitempty"`
}

func (*WarrantyMetadata) DocumentType() DocumentType {
	return DocumentTypeWarranty
}

// EntityType represents the standardized type of an extracted entity.
type EntityType string

const (
	EntityTypePerson       EntityType = "person"
	EntityTypeOrganisation EntityType = "organisation"
	EntityTypeProduct      EntityType = "product"
	EntityTypeVehicle      EntityType = "vehicle"
	EntityTypeLocation     EntityType = "location"
)

// Entity represents a named entity extracted from a document.
type Entity struct {
	Type  EntityType `json:"type"`
	Value string     `json:"value"`
}
