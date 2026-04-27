package payments

import (
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"os"
)

// ChargeRequest represents an incoming payment authorization request.
type ChargeRequest struct {
	MerchantID    string  `json:"merchant_id"`
	AmountCents   int64   `json:"amount_cents"`
	Currency      string  `json:"currency"`
	CardToken     string  `json:"card_token"` // PCI scope — never accept raw PAN
	IdempotencyKey string `json:"idempotency_key"`
}

// ChargeResponse is what we send back to the merchant.
type ChargeResponse struct {
	TransactionID string `json:"transaction_id"`
	Status        string `json:"status"`
	AmountCents   int64  `json:"amount_cents"`
	Currency      string `json:"currency"`
}

// HandleCharge accepts a payment authorization and forwards it to the processor.
// PCI-scoped: every change here requires a senior + domain-owner approval (T3).
func HandleCharge(w http.ResponseWriter, r *http.Request) {
	var req ChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := validateCharge(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	processorKey := os.Getenv("PAYMENT_PROCESSOR_API_KEY")
	if processorKey == "" {
		http.Error(w, "payment processor unavailable", http.StatusServiceUnavailable)
		return
	}

	resp, err := submitToProcessor(processorKey, &req)
	if err != nil {
		http.Error(w, "payment authorization failed", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func validateCharge(req *ChargeRequest) error {
	if req.MerchantID == "" {
		return errors.New("merchant_id required")
	}
	if req.AmountCents <= 0 {
		return errors.New("amount_cents must be positive")
	}
	if req.AmountCents > 1_000_000_00 {
		return errors.New("amount exceeds per-transaction limit")
	}
	if req.Currency != "USD" {
		return errors.New("only USD supported in pilot")
	}
	if req.IdempotencyKey == "" {
		return errors.New("idempotency_key required for retry safety")
	}
	return nil
}

// roundToMinor enforces 2-decimal monetary precision before processor handoff.
// Subtle: a bug here is a payment-correctness bug — strict T3 territory.
func roundToMinor(amount float64) int64 {
	return int64(math.Round(amount * 100))
}
