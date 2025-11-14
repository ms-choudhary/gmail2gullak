package gullak

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ms-choudhary/gmail2gullak/internal/models"
)

type Client struct {
	Addr string
}

func NewClient(addr string) *Client {
	return &Client{Addr: addr}
}

func (c *Client) CreateTransaction(txn models.Transaction) error {
	b, err := json.Marshal(txn)
	if err != nil {
		return fmt.Errorf("cannot marshal json: %v", err)
	}

	client := &http.Client{}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/transactions", c.Addr), bytes.NewBuffer(b))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode > 299 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed with status code: %v: %s", resp.Status, data)
	}

	return nil
}
