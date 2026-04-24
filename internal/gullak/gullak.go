package gullak

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ms-choudhary/gmail2gullak/internal/models"
)

type Client struct {
	Addr string
}

func NewClient(addr string) *Client {
	return &Client{Addr: addr}
}

func (c *Client) Match(msg models.Message) bool {
	_, err := ParseTransaction(msg)
	return err != NotTransactionErr
}

func (c *Client) Handle(message models.Message) error {
	txn, err := ParseTransaction(message)
	if err != nil {
		return err
	}

	if err := c.CreateTransaction(txn); err != nil {
		return err
	}

	return nil
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

	log.Printf("creating transaction: %v", txn)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %v", err)
	}

	type Resp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}

	var r Resp

	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return fmt.Errorf("failed to decode resp body: %v", err)
	}

	if resp.StatusCode > 299 {
		return fmt.Errorf("failed with status code: %v: %s", resp.Status, r.Error)
	}

	log.Printf("message: %s", r.Message)

	return nil
}
