package httpd

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"payment_backend/internal/domain"
	"payment_backend/internal/repository"
	"payment_backend/internal/usecase"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-playground/validator/v10"
)

type Handler struct {
	uc       *usecase.QRUsecase
	repo     *repository.SQLiteRepo
	validate *validator.Validate
}

func NewHandler(uc *usecase.QRUsecase, repo *repository.SQLiteRepo) *Handler {
	return &Handler{
		uc:       uc,
		repo:     repo,
		validate: validator.New(),
	}
}

func (h *Handler) Routes(sig SigConfig) http.Handler {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:5173"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Use(SignatureMiddleware(sig))

	r.Post("/api/v1/qr/generate", h.GenerateQR)
	r.Post("/api/v1/qr/payment", h.PaymentCallback)
	r.Get("/api/v1/transactions", h.ListTransactions)
	r.Get("/api/v1/transactions/{referenceNo}", h.GetTransaction)
	r.Get("/api/v1/healthz", h.Healthz)

	return r
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func parseAmountToMinor(value string) (int64, error) {
	r := new(big.Rat)
	_, ok := r.SetString(value)
	if !ok {
		return 0, ErrBadAmount
	}

	r.Mul(r, big.NewRat(100, 1))
	i := new(big.Int)
	i.Div(r.Num(), r.Denom())

	return i.Int64(), nil
}

var ErrBadAmount = &apiErr{Status: http.StatusBadRequest, Msg: "invalid amount format"}

type apiErr struct {
	Status int
	Msg    string
}

func (e *apiErr) Error() string { return e.Msg }

func (h *Handler) GenerateQR(w http.ResponseWriter, r *http.Request) {
	var req GenerateQRReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	if err := h.validate.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	amountMinor, err := parseAmountToMinor(req.Amount.Value)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if amountMinor <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount must be > 0"})
		return
	}

	tx, qr, err := h.uc.GenerateQR(r.Context(), req.MerchantID, req.PartnerReferenceNo, amountMinor, req.Amount.Currency)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	resp := GenerateQRResp{
		ResponseCode:       "2004700",
		ResponseMessage:    "Successful",
		ReferenceNo:        tx.ReferenceNo,
		PartnerReferenceNo: tx.PartnerReferenceNo,
		QRContent:          qr,
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /api/v1/qr/payment
func (h *Handler) PaymentCallback(w http.ResponseWriter, r *http.Request) {
	var req PaymentCallbackReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if err := h.validate.Struct(req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var status domain.TxStatus = domain.StatusPending
	switch req.TransactionStatusDesc {
	case "Success":
		status = domain.StatusSuccess
	case "Failed":
		status = domain.StatusFailed
	case "Pending":
		status = domain.StatusPending
	}

	if _, err := parseAmountToMinor(req.Amount.Value); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	err := h.uc.PaymentCallback(r.Context(), req.OriginalReferenceNo, status, req.PaidTime)
	if err != nil {
		if err == repository.ErrNotFound {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "transaction not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	resp := PaymentCallbackResp{
		ResponseCode:          "2005100",
		ResponseMessage:       "Successful",
		TransactionStatusDesc: req.TransactionStatusDesc,
	}
	writeJSON(w, http.StatusOK, resp)
}

// GET /api/v1/transactions?merchantId=&referenceNo=&partnerReferenceNo=&status=&limit=&offset=
func (h *Handler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := repository.TxFilter{
		MerchantID:         q.Get("merchantId"),
		ReferenceNo:        q.Get("referenceNo"),
		PartnerReferenceNo: q.Get("partnerReferenceNo"),
	}
	if st := q.Get("status"); st != "" {
		filter.Status = domain.TxStatus(st)
	}

	limit := 50
	offset := 0
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	items, err := h.repo.ListTransactions(r.Context(), filter, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	out := make([]TxItem, 0, len(items))
	for _, t := range items {
		out = append(out, toTxItem(t))
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /api/v1/transactions/{referenceNo}
func (h *Handler) GetTransaction(w http.ResponseWriter, r *http.Request) {
	ref := chi.URLParam(r, "referenceNo")
	t, err := h.repo.GetByReferenceNo(r.Context(), ref)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "transaction not found"})
		return
	}

	writeJSON(w, http.StatusOK, toTxItem(*t))
}

func toTxItem(t domain.Transaction) TxItem {
	return TxItem{
		ReferenceNo:        t.ReferenceNo,
		PartnerReferenceNo: t.PartnerReferenceNo,
		MerchantID:         t.MerchantID,
		AmountString:       formatMinorToString(t.AmountValueMinor),
		Currency:           t.Currency,
		Status:             string(t.Status),
		TransactionDate:    t.TransactionDate,
		PaidDate:           t.PaidDate,
	}
}

func formatMinorToString(minor int64) string {
	sign := ""
	if minor < 0 {
		sign = "-"
		minor = -minor
	}

	intPart := minor / 100
	decPart := minor % 100
	return sign + strconv.FormatInt(intPart, 10) + "." + twoDigits(int(decPart))
}

func twoDigits(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 5*time.Second)
}
