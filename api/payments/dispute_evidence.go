package payments

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// EvidenceType is the kind of evidence the merchant is attaching to a dispute.
// Each type maps to a different field set the network expects.
type EvidenceType string

const (
	EvidenceShippingProof   EvidenceType = "shipping_proof"
	EvidenceServiceLog      EvidenceType = "service_log"
	EvidenceCommunication   EvidenceType = "communication"
	EvidenceRefundIssued    EvidenceType = "refund_issued"
	EvidenceCancellationLog EvidenceType = "cancellation_log"
)

// EvidencePayload bundles all the merchant-supplied artifacts that we
// translate into the network's evidence schema before submission.
type EvidencePayload struct {
	DisputeID         string         `json:"dispute_id"`
	Type              EvidenceType   `json:"type"`
	IdempotencyKey    string         `json:"idempotency_key"`
	ShippingCarrier   string         `json:"shipping_carrier,omitempty"`
	TrackingNumber    string         `json:"tracking_number,omitempty"`
	DeliveryDate      *time.Time     `json:"delivery_date,omitempty"`
	ServiceLogURL     string         `json:"service_log_url,omitempty"`
	EmailThreadIDs    []string       `json:"email_thread_ids,omitempty"`
	RefundReceiptID   string         `json:"refund_receipt_id,omitempty"`
	AdditionalContext string         `json:"additional_context,omitempty"`
	SubmittedFields   map[string]any `json:"submitted_fields,omitempty"`
}

// HandleSubmitEvidence accepts a merchant's evidence package and forwards it
// to the processor's chargeback evidence API. Idempotent on IdempotencyKey.
//
// PCI-scoped (T3) — money outcome depends on this submission. A botched
// submission means the merchant loses the dispute and eats the chargeback.
func HandleSubmitEvidence(w http.ResponseWriter, r *http.Request) {
	var payload EvidencePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := validateEvidence(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dispute, found := disputeStore[payload.DisputeID]
	if !found {
		http.Error(w, "dispute not found", http.StatusNotFound)
		return
	}

	if dispute.Status == StatusWon || dispute.Status == StatusLost || dispute.Status == StatusWithdrawn {
		http.Error(w, "dispute is already resolved — cannot submit additional evidence", http.StatusConflict)
		return
	}

	if time.Now().UTC().After(dispute.EvidenceDueBy) {
		http.Error(w, "evidence deadline has passed", http.StatusGone)
		return
	}

	mapped, err := mapEvidenceToNetworkSchema(&dispute, &payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("could not map evidence: %v", err), http.StatusBadRequest)
		return
	}

	if err := submitEvidenceToProcessor(dispute.NetworkProcessor, mapped); err != nil {
		http.Error(w, "evidence submission failed", http.StatusBadGateway)
		return
	}

	dispute.Status = StatusEvidenceReady
	disputeStore[payload.DisputeID] = dispute

	respondJSON(w, http.StatusOK, &dispute)
}

func validateEvidence(p *EvidencePayload) error {
	if strings.TrimSpace(p.DisputeID) == "" {
		return errors.New("dispute_id is required")
	}
	if p.IdempotencyKey == "" {
		return errors.New("idempotency_key is required")
	}
	switch p.Type {
	case EvidenceShippingProof:
		if p.ShippingCarrier == "" || p.TrackingNumber == "" {
			return errors.New("shipping_proof requires shipping_carrier and tracking_number")
		}
	case EvidenceServiceLog:
		if p.ServiceLogURL == "" {
			return errors.New("service_log requires service_log_url")
		}
	case EvidenceCommunication:
		if len(p.EmailThreadIDs) == 0 {
			return errors.New("communication requires at least one email_thread_id")
		}
	case EvidenceRefundIssued:
		if p.RefundReceiptID == "" {
			return errors.New("refund_issued requires refund_receipt_id")
		}
	case EvidenceCancellationLog:
		// no extra requirements — additional_context is the surface here
	default:
		return fmt.Errorf("evidence type %q is not recognized", p.Type)
	}
	return nil
}

// mapEvidenceToNetworkSchema converts our merchant-friendly payload into the
// shape the network/processor's API expects. Different processors want
// different field names — Stripe uses snake_case, Adyen uses camelCase.
func mapEvidenceToNetworkSchema(d *DisputeRecord, p *EvidencePayload) (map[string]any, error) {
	switch d.NetworkProcessor {
	case "stripe":
		return mapStripeEvidence(p), nil
	case "adyen":
		return mapAdyenEvidence(p), nil
	case "braintree":
		return mapBraintreeEvidence(p), nil
	default:
		return nil, fmt.Errorf("unsupported processor: %s", d.NetworkProcessor)
	}
}

func mapStripeEvidence(p *EvidencePayload) map[string]any {
	out := map[string]any{
		"dispute":         p.DisputeID,
		"idempotency_key": p.IdempotencyKey,
	}
	switch p.Type {
	case EvidenceShippingProof:
		out["shipping_carrier"] = p.ShippingCarrier
		out["shipping_tracking_number"] = p.TrackingNumber
		if p.DeliveryDate != nil {
			out["shipping_date"] = p.DeliveryDate.Format(time.RFC3339)
		}
	case EvidenceServiceLog:
		out["service_documentation"] = p.ServiceLogURL
	case EvidenceCommunication:
		out["customer_communication"] = strings.Join(p.EmailThreadIDs, ",")
	case EvidenceRefundIssued:
		out["refund_policy_disclosure"] = p.RefundReceiptID
	case EvidenceCancellationLog:
		out["cancellation_policy_disclosure"] = p.AdditionalContext
	}
	if p.AdditionalContext != "" {
		out["uncategorized_text"] = p.AdditionalContext
	}
	return out
}

func mapAdyenEvidence(p *EvidencePayload) map[string]any {
	out := map[string]any{
		"disputeId":      p.DisputeID,
		"idempotencyKey": p.IdempotencyKey,
	}
	switch p.Type {
	case EvidenceShippingProof:
		out["shippingCarrier"] = p.ShippingCarrier
		out["trackingNumber"] = p.TrackingNumber
	case EvidenceServiceLog:
		out["serviceDocumentation"] = p.ServiceLogURL
	case EvidenceCommunication:
		out["customerCommunication"] = p.EmailThreadIDs
	case EvidenceRefundIssued:
		out["refundReceiptId"] = p.RefundReceiptID
	case EvidenceCancellationLog:
		out["cancellationPolicyText"] = p.AdditionalContext
	}
	return out
}

func mapBraintreeEvidence(p *EvidencePayload) map[string]any {
	return map[string]any{
		"disputeId":      p.DisputeID,
		"idempotencyKey": p.IdempotencyKey,
		"category":       string(p.Type),
		"context":        p.AdditionalContext,
	}
}

// submitEvidenceToProcessor is a stub for the demo. In production this calls
// the actual SDK and surfaces a structured error on retryable vs terminal
// network failures.
func submitEvidenceToProcessor(processor string, payload map[string]any) error {
	// In production: real HTTP/SDK call with retries, exponential backoff,
	// and a circuit breaker. Here, just no-op.
	_ = processor
	_ = payload
	return nil
}
