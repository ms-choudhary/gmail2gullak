package models

import "fmt"

type Message struct {
	Subject        string          `json:"subject"`
	Body           string          `json:"body"`
	From           string          `json:"from"`
	Date           string          `json:"date"`
	PDFAttachments []PDFAttachment `json:"pdf_attachments"`
}

type PDFAttachment struct {
	Filename     string `json:"filename"`
	MimeType     string `json:"mime_type"`
	AttachmentID string `json:"attachment_id"`
}

type Transaction struct {
	Amount          float64 `json:"amount"`
	Description     string  `json:"description"`
	TransactionDate string  `json:"transaction_date"`
}

func (t Transaction) String() string {
	return fmt.Sprintf("Amount: %f, Description: %s, Date: %s", t.Amount, t.Description, t.TransactionDate)
}
