package parser

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ms-choudhary/gmail2gullak/internal/models"
)

type Parser struct {
	priceRegex  *regexp.Regexp
	vendorRegex *regexp.Regexp
}

var (
	hdfcUPIParser = Parser{
		priceRegex:  regexp.MustCompile(`Rs\.(\d+(?:\.\d+)?) has been debited`),
		vendorRegex: regexp.MustCompile(`to VPA\s+\S+\s+(.+?)\s+on\s+`),
	}

	hdfcCreditCardParser = Parser{
		priceRegex:  regexp.MustCompile(`Rs\.(\d+(?:\.\d+)?) is debited from`),
		vendorRegex: regexp.MustCompile(`towards\s+([^\s]+(?:\s+[^\s]+)*?)\s+on\s+`),
	}
)

var NotTransactionErr = errors.New("not a transaction")

func IsNotTransaction(err error) bool {
	return err == NotTransactionErr
}

func (p Parser) parse(msg models.Message) (models.Transaction, error) {
	txn := models.Transaction{}

	priceMatch := p.priceRegex.FindStringSubmatch(msg.Body)
	if len(priceMatch) > 1 {
		amount, err := strconv.ParseFloat(priceMatch[1], 64)
		if err != nil {
			return models.Transaction{}, fmt.Errorf("failed to parse amount: %v", err)
		}

		txn.Amount = amount
	}

	vendorMatch := p.vendorRegex.FindStringSubmatch(msg.Body)
	if len(vendorMatch) > 1 {
		txn.Description = strings.TrimSpace(vendorMatch[1])
	}

	if txn.Amount == 0 || txn.Description == "" {
		return models.Transaction{}, fmt.Errorf("failed to parse transaction details: amount: %v, description: %s", txn.Amount, txn.Description)
	}

	date, err := parseDate(msg.Date)
	if err != nil {
		return models.Transaction{}, err
	}

	txn.TransactionDate = date

	return txn, nil
}

func parseDate(datestr string) (string, error) {
	inputLayout := "Mon, 02 Jan 2006 15:04:05 -0700"
	parsedDate, err := time.Parse(inputLayout, datestr)
	if err != nil {
		return "", fmt.Errorf("failed to parse date: %s: %v", datestr, err)
	}
	return parsedDate.Format("2006-01-02"), nil
}

func ParseTransaction(msg models.Message) (models.Transaction, error) {
	if strings.Contains(msg.Subject, "You have done a UPI txn") {
		return hdfcUPIParser.parse(msg)
	} else if strings.Contains(msg.Subject, "debited via Credit Card") {
		return hdfcCreditCardParser.parse(msg)
	}
	return models.Transaction{}, NotTransactionErr
}
