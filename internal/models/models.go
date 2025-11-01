package models

import "fmt"

type Message struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
	From    string `json:"from"`
	Date    string `json:"date"`
}

type Transaction struct {
	Amount          float64 `json:"amount"`
	Description     string  `json:"description"`
	TransactionDate string  `json:"transaction_date"`
}

func (t Transaction) String() string {
	return fmt.Sprintf("Amount: %f, Description: %s, Date: %s", t.Amount, t.Description, t.TransactionDate)
}
