package notifications

import (
	"errors"
	"os"
	"regexp"
)

var phoneE164 = regexp.MustCompile(`^\+[1-9]\d{1,14}$`)

// SendOrderReceipt sends an order confirmation SMS to the merchant's customer.
// T2 — non-PCI backend, 1 human approver required.
func SendOrderReceipt(phoneNumber, orderID string, totalCents int64) error {
	if !phoneE164.MatchString(phoneNumber) {
		return errors.New("phone must be E.164 format")
	}
	provider := os.Getenv("SMS_PROVIDER_API_KEY")
	if provider == "" {
		return errors.New("SMS provider not configured")
	}
	body := formatReceipt(orderID, totalCents)
	return sendSMS(provider, phoneNumber, body)
}

func formatReceipt(orderID string, totalCents int64) string {
	return "Thanks! Order " + orderID + " confirmed for $" + formatDollars(totalCents) + ". Reply STOP to opt out."
}

func formatDollars(cents int64) string {
	dollars := cents / 100
	rem := cents % 100
	out := ""
	if dollars >= 1000 {
		out = string(rune('0'+dollars/1000)) + "," + padInt(int(dollars%1000), 3)
	} else {
		out = padInt(int(dollars), 0)
	}
	return out + "." + padInt(int(rem), 2)
}

func padInt(n, width int) string {
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	for len(s) < width {
		s = "0" + s
	}
	if s == "" {
		s = "0"
	}
	return s
}

func sendSMS(apiKey, to, body string) error { return nil }
