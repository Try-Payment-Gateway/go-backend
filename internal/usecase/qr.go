package usecase

import (
	"context"
	"fmt"
	"payment_backend/internal/domain"
	"payment_backend/internal/repository"
	"time"

	"github.com/google/uuid"
)

type QRUsecase struct {
	repo *repository.SQLiteRepo
}

func NewQRUsecase(r *repository.SQLiteRepo) *QRUsecase {
	return &QRUsecase{repo: r}
}

func (u *QRUsecase) GenerateQR(ctx context.Context, merchantId, partnerRef string, amountMinor int64, currency string) (*domain.Transaction, string, error) {
	if amountMinor <= 0 {
		return nil, "", fmt.Errorf("amount must be > 0")
	}

	refNo := "A" + uuid.New().String()[:10]

	tx := &domain.Transaction{
		ReferenceNo:        refNo,
		PartnerReferenceNo: partnerRef,
		MerchantID:         merchantId,
		AmountValueMinor:   amountMinor,
		Currency:           currency,
		Status:             domain.StatusCreated,
		TransactionDate:    time.Now(),
	}

	if err := u.repo.InsertTransaction(ctx, tx); err != nil {
		return nil, "", err
	}

	qrContent := "00020101021226620015" + refNo
	return tx, qrContent, nil
}

func (u *QRUsecase) PaymentCallback(ctx context.Context, originalRef string, status domain.TxStatus, paidAt *time.Time) error {
	_, err := u.repo.GetByReferenceNo(ctx, originalRef)
	if err != nil {
		return repository.ErrNotFound
	}

	return u.repo.UpdatePaymentStatus(ctx, originalRef, status, paidAt)
}
