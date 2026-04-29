package payments

import (
	"strings"
	"testing"
	"time"
)

func TestSanitizeNote_RedactsLongDigitRuns(t *testing.T) {
	in := "customer card was 4111111111111111 and SSN 123456789"
	got := sanitizeNote(in)
	if strings.Contains(got, "4111111111111111") {
		t.Fatalf("expected card number to be redacted, got: %q", got)
	}
	if strings.Contains(got, "123456789") {
		t.Fatalf("expected SSN-like run to be redacted, got: %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("expected redaction marker, got: %q", got)
	}
}

func TestSanitizeNote_KeepsShortNumbers(t *testing.T) {
	in := "order #1234 was for $50"
	got := sanitizeNote(in)
	if got != in {
		t.Fatalf("short numeric tokens should be preserved, got: %q", got)
	}
}

func TestEvidenceDeadline_FraudShorterThanGeneral(t *testing.T) {
	now := time.Now().UTC()
	fraud := evidenceDeadline(ReasonFraudulent, now)
	general := evidenceDeadline(ReasonGeneral, now)
	if !fraud.Before(general) {
		t.Fatalf("fraud deadline should be earlier than general; fraud=%v general=%v", fraud, general)
	}
}

func TestValidateDispute_RejectsMissingFields(t *testing.T) {
	cases := []struct {
		name string
		req  DisputeRequest
		want string
	}{
		{"no transaction id", DisputeRequest{IdempotencyKey: "k", AmountCents: 100, Currency: "USD", Reason: ReasonGeneral}, "transaction_id"},
		{"no idempotency",    DisputeRequest{TransactionID: "t", AmountCents: 100, Currency: "USD", Reason: ReasonGeneral}, "idempotency_key"},
		{"non-positive amt", DisputeRequest{TransactionID: "t", IdempotencyKey: "k", AmountCents: 0, Currency: "USD", Reason: ReasonGeneral}, "amount_cents"},
		{"unknown ccy",      DisputeRequest{TransactionID: "t", IdempotencyKey: "k", AmountCents: 100, Currency: "ZZZ", Reason: ReasonGeneral}, "currency"},
		{"unknown reason",   DisputeRequest{TransactionID: "t", IdempotencyKey: "k", AmountCents: 100, Currency: "USD", Reason: "bogus"}, "reason"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateDispute(&tc.req)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error containing %q, got: %v", tc.want, err)
			}
		})
	}
}

func TestDetectProcessor_KnownPrefixes(t *testing.T) {
	cases := map[string]string{
		"ch_st_abc": "stripe",
		"ch_aj_xyz": "adyen",
		"ch_br_qrs": "braintree",
		"ch_zz_???": "unknown",
	}
	for in, want := range cases {
		if got := detectProcessor(in); got != want {
			t.Errorf("detectProcessor(%q) = %q, want %q", in, got, want)
		}
	}
}
