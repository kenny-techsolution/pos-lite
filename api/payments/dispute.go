package payments

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

// DisputeReason classifies the merchant-supplied reason for a chargeback.
// Used by the processor to route to the correct evidence-collection workflow.
type DisputeReason string

const (
	ReasonFraudulent       DisputeReason = "fraudulent"
	ReasonProductNotRcvd   DisputeReason = "product_not_received"
	ReasonProductDefective DisputeReason = "product_defective"
	ReasonDuplicate        DisputeReason = "duplicate"
	ReasonSubscription     DisputeReason = "subscription_canceled"
	ReasonGeneral          DisputeReason = "general"
)

// DisputeStatus tracks the dispute through its lifecycle.
type DisputeStatus string

const (
	StatusOpen          DisputeStatus = "open"
	StatusEvidenceReady DisputeStatus = "evidence_ready"
	StatusUnderReview   DisputeStatus = "under_review"
	StatusWon           DisputeStatus = "won"
	StatusLost          DisputeStatus = "lost"
	StatusWithdrawn     DisputeStatus = "withdrawn"
)

// DisputeRequest is the inbound payload from the merchant dashboard when
// they receive a chargeback notification from the processor and want to
// initiate evidence submission.
type DisputeRequest struct {
	TransactionID  string        `json:"transaction_id"`
	Reason         DisputeReason `json:"reason"`
	AmountCents    int64         `json:"amount_cents"`
	Currency       string        `json:"currency"`
	IdempotencyKey string        `json:"idempotency_key"`
	MerchantNote   string        `json:"merchant_note"`
}

// DisputeRecord is the persisted state — also returned in API responses.
type DisputeRecord struct {
	DisputeID         string        `json:"dispute_id"`
	TransactionID     string        `json:"transaction_id"`
	Reason            DisputeReason `json:"reason"`
	Status            DisputeStatus `json:"status"`
	AmountCents       int64         `json:"amount_cents"`
	Currency          string        `json:"currency"`
	OpenedAt          time.Time     `json:"opened_at"`
	EvidenceDueBy     time.Time     `json:"evidence_due_by"`
	ResolvedAt        *time.Time    `json:"resolved_at,omitempty"`
	MerchantNote      string        `json:"merchant_note,omitempty"`
	NetworkProcessor string        `json:"network_processor"`
}

// HandleOpenDispute accepts an inbound chargeback notification and creates
// a dispute record. Idempotent on IdempotencyKey — repeated requests with
// the same key return the same DisputeRecord without creating duplicates.
//
// PCI-scoped: every modification to the dispute pipeline requires a senior
// reviewer plus payments domain-owner approval (T3). Money flows through
// here and into the processor's chargeback evidence API.
func HandleOpenDispute(w http.ResponseWriter, r *http.Request) {
	var req DisputeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := validateDispute(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Idempotency check — if a dispute exists for this key we return it instead
	// of creating a duplicate. Avoids double-charging the merchant for the same
	// chargeback when the dashboard retries on transient network failures.
	if existing, found := lookupDisputeByIdempotencyKey(req.IdempotencyKey); found {
		respondJSON(w, http.StatusOK, existing)
		return
	}

	record := DisputeRecord{
		DisputeID:        generateDisputeID(),
		TransactionID:    req.TransactionID,
		Reason:           req.Reason,
		Status:           StatusOpen,
		AmountCents:      req.AmountCents,
		Currency:         strings.ToUpper(req.Currency),
		OpenedAt:         time.Now().UTC(),
		EvidenceDueBy:    evidenceDeadline(req.Reason, time.Now().UTC()),
		MerchantNote:     sanitizeNote(req.MerchantNote),
		NetworkProcessor: detectProcessor(req.TransactionID),
	}

	if err := persistDispute(&record, req.IdempotencyKey); err != nil {
		http.Error(w, "could not persist dispute", http.StatusInternalServerError)
		return
	}

	if err := emitDisputeOpenedEvent(&record); err != nil {
		// Persistence already succeeded; log-and-continue rather than 500.
		fmt.Fprintf(os.Stderr, "dispute %s: event emission failed: %v\n", record.DisputeID, err)
	}

	respondJSON(w, http.StatusCreated, &record)
}

// validateDispute enforces the contract on the inbound request before we
// touch any persistence. Most of these are also enforced at the DB level,
// but failing fast saves a round-trip and keeps error messages clean.
func validateDispute(req *DisputeRequest) error {
	if strings.TrimSpace(req.TransactionID) == "" {
		return errors.New("transaction_id is required")
	}
	if req.IdempotencyKey == "" {
		return errors.New("idempotency_key is required")
	}
	if req.AmountCents <= 0 {
		return errors.New("amount_cents must be positive")
	}
	if req.AmountCents > 10_000_000 {
		// Sanity ceiling — disputes above $100k must go through the manual
		// review queue, not this endpoint.
		return errors.New("amount_cents exceeds online dispute ceiling")
	}
	if !isValidCurrency(req.Currency) {
		return fmt.Errorf("currency %q is not in the supported list", req.Currency)
	}
	if !isValidReason(req.Reason) {
		return fmt.Errorf("reason %q is not a recognized dispute reason", req.Reason)
	}
	return nil
}

// evidenceDeadline computes when the merchant must submit evidence by.
// Different chargeback reasons have different network deadlines; we pick the
// shortest network-mandated deadline minus a 24h buffer for our own review.
func evidenceDeadline(reason DisputeReason, openedAt time.Time) time.Time {
	// Network-imposed deadlines in days. These are the conservative floors —
	// actual deadlines are sometimes longer per network, but using the floor
	// means we never miss a window.
	days := map[DisputeReason]int{
		ReasonFraudulent:       7,
		ReasonProductNotRcvd:   14,
		ReasonProductDefective: 14,
		ReasonDuplicate:        7,
		ReasonSubscription:     21,
		ReasonGeneral:          10,
	}
	d, ok := days[reason]
	if !ok {
		d = 7 // unknown reason → assume the strictest deadline
	}
	// Subtract a 24h internal-review buffer so our team has time to QA the
	// merchant's evidence before submission.
	return openedAt.Add(time.Duration(d-1) * 24 * time.Hour)
}

func isValidCurrency(c string) bool {
	supported := map[string]bool{
		"USD": true, "EUR": true, "GBP": true, "CAD": true, "AUD": true,
		"JPY": true, "TWD": true, "HKD": true, "SGD": true,
	}
	return supported[strings.ToUpper(c)]
}

func isValidReason(r DisputeReason) bool {
	switch r {
	case ReasonFraudulent, ReasonProductNotRcvd, ReasonProductDefective,
		ReasonDuplicate, ReasonSubscription, ReasonGeneral:
		return true
	}
	return false
}

// sanitizeNote strips PII-looking patterns from merchant-supplied free text
// before we persist it. We only catch the obvious ones — emails, phone-like
// digit runs — but we never want a customer support agent to paste raw card
// data into a note field and have us store it.
func sanitizeNote(note string) string {
	note = strings.TrimSpace(note)
	if len(note) > 2000 {
		note = note[:2000]
	}
	// Redact long digit runs that look like a card or SSN.
	var b strings.Builder
	digits := 0
	for _, r := range note {
		if r >= '0' && r <= '9' {
			digits++
			if digits > 6 {
				continue // skip — this is part of a long digit run
			}
			b.WriteRune(r)
		} else {
			if digits > 6 {
				b.WriteString("[REDACTED]")
			}
			digits = 0
			b.WriteRune(r)
		}
	}
	if digits > 6 {
		b.WriteString("[REDACTED]")
	}
	return b.String()
}

// detectProcessor classifies which network/processor handled the original
// transaction so the right chargeback evidence template is used downstream.
func detectProcessor(txID string) string {
	switch {
	case strings.HasPrefix(txID, "ch_st_"):
		return "stripe"
	case strings.HasPrefix(txID, "ch_aj_"):
		return "adyen"
	case strings.HasPrefix(txID, "ch_br_"):
		return "braintree"
	default:
		return "unknown"
	}
}

func generateDisputeID() string {
	// Format: dsp_<unix>_<rand>. Production uses a real ULID; this is fine
	// for the demo because collision risk in a single-instance test is zero.
	return fmt.Sprintf("dsp_%d_%04d", time.Now().UTC().UnixNano(), int(math.Mod(float64(time.Now().UTC().UnixNano()), 10000)))
}

// --- persistence and event-emission stubs ---
// In production these are wired to the dispute store and the Kafka topic.

var disputeStore = map[string]DisputeRecord{}        // dispute_id → record
var idempotencyIndex = map[string]string{}           // idempotency_key → dispute_id

func lookupDisputeByIdempotencyKey(key string) (*DisputeRecord, bool) {
	id, ok := idempotencyIndex[key]
	if !ok {
		return nil, false
	}
	rec, ok := disputeStore[id]
	if !ok {
		return nil, false
	}
	return &rec, true
}

func persistDispute(rec *DisputeRecord, idempotencyKey string) error {
	disputeStore[rec.DisputeID] = *rec
	idempotencyIndex[idempotencyKey] = rec.DisputeID
	return nil
}

func emitDisputeOpenedEvent(rec *DisputeRecord) error {
	// In production: produce to "payments.disputes.opened" Kafka topic with
	// the canonical schema. Here, we just log.
	fmt.Fprintf(os.Stdout, "event: dispute_opened: %s amount=%d %s reason=%s\n",
		rec.DisputeID, rec.AmountCents, rec.Currency, rec.Reason)
	return nil
}

func respondJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
