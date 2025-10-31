package domain

import "time"

type TxStatus string

const (
	StatusCreated TxStatus = "CREATED"
	StatusSuccess TxStatus = "SUCCESS"
	StatusFailed  TxStatus = "FAILED"
	StatusPending TxStatus = "PENDING"
)

type Transaction struct {
	ID                 int64
	ReferenceNo        string
	PartnerReferenceNo string
	MerchantID         string
	AmountValueMinor   int64
	Currency           string
	Status             TxStatus
	TransactionDate    time.Time
	PaidDate           *time.Time
}
