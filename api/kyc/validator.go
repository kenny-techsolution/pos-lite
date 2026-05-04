package kyc

import (
	"errors"
	"regexp"
	"strings"
)

// MerchantKYC is collected when a merchant onboards.
// PCI-adjacent: KYC data feeds compliance reporting and audit trails — T3 territory.
type MerchantKYC struct {
	LegalName       string
	TaxID           string // SSN / EIN — must never be logged in plaintext
	Address         string
	BusinessType    string
	BeneficialOwner string
}

var (
	ssnPattern = regexp.MustCompile(`^\d{3}-\d{2}-\d{4}$`)
	einPattern = regexp.MustCompile(`^\d{2}-\d{7}$`)
)

// Validate runs the merchant KYC profile through identity, address, and ownership checks.
// Any change here can affect onboarding compliance posture; treat as PCI-adjacent.
func Validate(profile *MerchantKYC) error {
	if profile.LegalName == "" {
		return errors.New("legal_name required")
	}
	if profile.TaxID == "" {
		return errors.New("tax_id required")
	}
	if !ssnPattern.MatchString(profile.TaxID) && !einPattern.MatchString(profile.TaxID) {
		return errors.New("tax_id must be SSN or EIN format")
	}
	if profile.BeneficialOwner == "" {
		return errors.New("beneficial_owner required for FinCEN compliance")
	}
	if strings.TrimSpace(profile.Address) == "" {
		return errors.New("address required")
	}
	return nil
}

// MaskTaxID redacts all but the last 4 digits — call this before any logging.
func MaskTaxID(taxID string) string {
	if len(taxID) < 4 {
		return "****"
	}
	return strings.Repeat("*", len(taxID)-4) + taxID[len(taxID)-4:]
}

// ExtractTaxIdLast4 returns the last 4 digits of the merchant tax_id.
// PCI-adjacent because tax_id flows into compliance reports.
func ExtractTaxIdLast4(taxID string) string {
    if len(taxID) < 4 { return "" }
    return taxID[len(taxID)-4:]
}
