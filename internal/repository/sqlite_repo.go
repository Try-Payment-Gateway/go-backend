package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"payment_backend/internal/domain"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteRepo struct {
	db *sql.DB
}

func NewSQLiteRepo(dsn string) (*SQLiteRepo, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.Exec("PRAGMA foreign_keys = ON;")
	db.Exec("PRAGMA journal_mode = WAL;")
	db.Exec("PRAGMA busy_timeout = 5000;")

	r := &SQLiteRepo{db: db}
	if err := r.migrate(); err != nil {
		return nil, err
	}

	return r, nil
}

func (r *SQLiteRepo) Close() error {
	return r.db.Close()
}

func (r *SQLiteRepo) migrate() error {
	schema := `
		CREATE TABLE IF NOT EXISTS transactions(
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			reference_no TEXT NOT NULL UNIQUE,
			partner_reference_no TEXT NOT NULL,
			merchant_id TEXT NOT NULL,
			amount_value_minor INTEGER NOT NULL,
			currency TEXT NOT NULL,
			status TEXT NOT NULL,
			"transaction_date" TEXT NOT NULL,
			"paid_date" TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_tx_reference_no ON transactions(reference_no);
		CREATE INDEX IF NOT EXISTS idx_tx_partner_reference_no ON transactions(partner_reference_no);
		CREATE INDEX IF NOT EXISTS idx_tx_merchant ON transactions(merchant_id); 
	`
	_, err := r.db.Exec(schema)
	return err
}

func (r *SQLiteRepo) InsertTransaction(ctx context.Context, t *domain.Transaction) error {
	q := `
		INSERT INTO transactions(
			reference_no, 
			partner_reference_no,
			merchant_id, 
			amount_value_minor, 
			currency, 
			status,
			transaction_date,
			paid_date
		)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?);
	`

	_, err := r.db.ExecContext(
		ctx, q,
		t.ReferenceNo,
		t.PartnerReferenceNo,
		t.MerchantID,
		t.AmountValueMinor,
		t.Currency,
		string(t.Status),
		t.TransactionDate.UTC().Format(time.RFC3339Nano),
		nil,
	)

	return err
}

func (r *SQLiteRepo) GetByReferenceNo(ctx context.Context, ref string) (*domain.Transaction, error) {
	q := `
		SELECT 
			id,
			reference_no,
			partner_reference_no,
			merchant_id,
			amount_value_minor,
			currency,
			status,
			transaction_date,
			paid_date
		FROM transactions WHERE reference_no = ?
	`

	row := r.db.QueryRowContext(ctx, q, ref)
	return scanTx(row)
}

func (r *SQLiteRepo) UpdatePaymentStatus(ctx context.Context, ref string, status domain.TxStatus, paid *time.Time) error {
	q := `UPDATE transactions SET status = ?, paid_date = ? WHERE reference_no = ?`
	var paidStr any = nil

	if paid != nil {
		paidStr = paid.UTC().Format(time.RFC3339Nano)
	}

	res, err := r.db.ExecContext(ctx, q, string(status), paidStr, ref)
	if err != nil {
		return err
	}

	aff, _ := res.RowsAffected()
	if aff == 0 {
		return sql.ErrNoRows
	}

	return nil
}

type TxFilter struct {
	MerchantID         string
	ReferenceNo        string
	PartnerReferenceNo string
	Status             domain.TxStatus
}

func (r *SQLiteRepo) ListTransactions(ctx context.Context, f TxFilter, limit, offset int) ([]domain.Transaction, error) {
	q := `
		SELECT
			id,
			reference_no,
			partner_reference_no,
			merchant_id,
			amount_value_minor,
			currency,
			status,
			transaction_date,
			paid_date
		FROM transactions WHERE 1 = 1
	`
	args := []any{}

	if f.MerchantID != "" {
		q += " AND merchant_id = ?"
		args = append(args, f.MerchantID)
	}

	if f.ReferenceNo != "" {
		q += "AND reference_no = ?"
		args = append(args, f.ReferenceNo)
	}

	if f.PartnerReferenceNo != "" {
		q += " AND partner_reference_no = ?"
		args = append(args, f.PartnerReferenceNo)
	}

	if f.Status != "" {
		q += " AND status = ?"
		args = append(args, string(f.Status))
	}

	q += " ORDER BY id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []domain.Transaction
	for rows.Next() {
		t, err := scanTx(rows)
		if err != nil {
			return nil, err
		}

		res = append(res, *t)
	}

	return res, nil
}

func scanTx(scanner interface {
	Scan(dest ...any) error
}) (*domain.Transaction, error) {
	var t domain.Transaction
	var status string
	var txDateStr string
	var paidStr *string

	if err := scanner.Scan(
		&t.ID,
		&t.ReferenceNo,
		&t.PartnerReferenceNo,
		&t.MerchantID,
		&t.AmountValueMinor,
		&t.Currency,
		&status,
		&txDateStr,
		&paidStr,
	); err != nil {
		return nil, err
	}

	t.Status = domain.TxStatus(status)

	txTime, err := time.Parse(time.RFC3339Nano, txDateStr)
	if err != nil {
		return nil, fmt.Errorf("parse tx time: %w", err)
	}

	t.TransactionDate = txTime
	if paidStr != nil {
		pd, err := time.Parse(time.RFC3339Nano, *paidStr)
		if err != nil {
			return nil, fmt.Errorf("parse paid time: %w", err)
		}

		t.PaidDate = &pd
	}

	return &t, nil
}

var ErrNotFound = errors.New("not found")
