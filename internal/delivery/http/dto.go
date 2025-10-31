package httpd

import "time"

type GenerateQRReq struct {
	PartnerReferenceNo string        `json:"partnerReferenceNo" validate:"required"`
	Amount             AmountPayload `json:"amount" validate:"required"`
	MerchantID         string        `json:"merchantId" validate:"required"`
}

type AmountPayload struct {
	Value    string `json:"value" validate:"required"`
	Currency string `json:"currency" validate:"required"`
}

type GenerateQRResp struct {
	ResponseCode       string `json:"responseCode"`
	ResponseMessage    string `json:"responseMessage"`
	ReferenceNo        string `json:"referenceNo"`
	PartnerReferenceNo string `json:"partnerReferenceNo"`
	QRContent          string `json:"qrContent"`
}

type PaymentCallbackReq struct {
	OriginalReferenceNo        string        `json:"originalReferenceNo" validate:"required"`
	OriginalPartnerReferenceNo string        `json:"originalPartnerReferenceNo" validate:"required"`
	TransactionStatusDesc      string        `json:"transactionStatusDesc" validate:"required"`
	PaidTime                   *time.Time    `json:"paidTime"`
	Amount                     AmountPayload `json:"amount" validate:"required"`
}

type PaymentCallbackResp struct {
	ResponseCode          string `json:"responseCode"`
	ResponseMessage       string `json:"responseMessage"`
	TransactionStatusDesc string `json:"transactionStatusDesc"`
}

type TxItem struct {
	ReferenceNo        string     `json:"referenceNo"`
	PartnerReferenceNo string     `json:"partnerReferenceNo"`
	MerchantID         string     `json:"merchantId"`
	AmountString       string     `json:"amount"`
	Currency           string     `json:"currency"`
	Status             string     `json:"status"`
	TransactionDate    time.Time  `json:"transactionDate"`
	PaidDate           *time.Time `json:"paidDate,omitempty"`
}
