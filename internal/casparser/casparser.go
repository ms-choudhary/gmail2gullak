package casparser

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"

	"github.com/ms-choudhary/gmail2gullak/internal/models"
)

type Client struct {
	Addr string
}

func NewClient(addr string) *Client {
	return &Client{Addr: addr}
}

func (c *Client) Match(msg models.Message) bool {
	if strings.Contains(msg.Subject, "Consolidated Account Statement - CAMS Mailback Request") {
		return true
	}
	return false
}

func (c *Client) Handle(msg models.Message) error {
	if len(msg.PDFAttachments) != 1 {
		return fmt.Errorf("expected 1 attachment, got: %v", len(msg.PDFAttachments))
	}

	attachment := msg.PDFAttachments[0]
	log.Printf("found 1 attachment, filename: %s", attachment.Filename)

	return c.sendToCASParser(attachment.FileData, attachment.Filename, "Linux11!")
}

func (c *Client) sendToCASParser(pdfData []byte, filename, password string) error {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}
	if _, err = io.Copy(part, bytes.NewReader(pdfData)); err != nil {
		return fmt.Errorf("writing PDF data: %w", err)
	}

	if err = writer.WriteField("password", password); err != nil {
		return fmt.Errorf("writing password field: %w", err)
	}

	writer.Close()

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/parse-cas", c.Addr), &buf)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("casparser returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
