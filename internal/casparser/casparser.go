package casparser

import (
	"bytes"
	"fmt"
	"io"
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

func (c *Client) Handle(msg models.Message) (string, error) {
	if len(msg.PDFAttachments) != 1 {
		return "", fmt.Errorf("[%s] casparser failed: expected 1 attachment, got: %v", msg.ID, len(msg.PDFAttachments))
	}

	attachment := msg.PDFAttachments[0]

	if err := c.sendToCASParser(attachment.FileData, attachment.Filename, "Linux11!"); err != nil {
		return "", fmt.Errorf("[%s] casparser api failed: %v", msg.ID, err)
	}

	return fmt.Sprintf("[%s] casparser cas statement %s added", msg.ID, attachment.Filename), nil
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
