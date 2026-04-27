package payments

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
)

type RefundRequest struct {
	OriginalTransactionID string `json:"original_transaction_id"`
	AmountCents           int64  `json:"amount_cents"`
	Reason                string `json:"reason"`
	IdempotencyKey        string `json:"idempotency_key"`
}

type RefundResponse struct {
	RefundID    string `json:"refund_id"`
	Status      string `json:"status"`
	AmountCents int64  `json:"amount_cents"`
}

// HandleRefund issues a partial or full refund against a previous charge.
// PCI-scoped: every change requires senior + domain-owner approval (T3).
func HandleRefund(w http.ResponseWriter, r *http.Request) {
	var req RefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if err := validateRefund(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	original, err := lookupOriginalCharge(req.OriginalTransactionID)
	if err != nil {
		http.Error(w, "original transaction not found", http.StatusNotFound)
		return
	}
	if req.AmountCents > original.AmountCents {
		http.Error(w, "refund exceeds original charge amount", http.StatusBadRequest)
		return
	}

	processorKey := os.Getenv("PAYMENT_PROCESSOR_API_KEY")
	resp, err := submitRefundToProcessor(processorKey, &req)
	if err != nil {
		http.Error(w, "refund failed", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func validateRefund(req *RefundRequest) error {
	if req.OriginalTransactionID == "" {
		return errors.New("original_transaction_id required")
	}
	if req.AmountCents <= 0 {
		return errors.New("amount_cents must be positive")
	}
	if req.IdempotencyKey == "" {
		return errors.New("idempotency_key required")
	}
	return nil
}

// internal helpers — would call payment processor SDK in production.
func submitToProcessor(apiKey string, req *ChargeRequest) (*ChargeResponse, error) {
	return &ChargeResponse{TransactionID: "txn_demo", Status: "approved", AmountCents: req.AmountCents, Currency: req.Currency}, nil
}
func submitRefundToProcessor(apiKey string, req *RefundRequest) (*RefundResponse, error) {
	return &RefundResponse{RefundID: "rfd_demo", Status: "succeeded", AmountCents: req.AmountCents}, nil
}
func lookupOriginalCharge(id string) (*ChargeResponse, error) {
	return &ChargeResponse{TransactionID: id, AmountCents: 100_00, Currency: "USD", Status: "approved"}, nil
}
