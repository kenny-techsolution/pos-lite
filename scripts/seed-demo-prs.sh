#!/usr/bin/env bash
# Seed the 6 demo PRs in this repo. Run once after the initial push to GitHub.
# Each branch gets one commit with a tier-illustrative file change, then opens a draft PR.
#
# Usage:
#   bash scripts/seed-demo-prs.sh
#
# Prerequisites:
#   - You're on the `main` branch with a clean working tree.
#   - `gh` CLI authenticated (`gh auth status`).
#   - Origin remote points at your GitHub fork of pos-lite.

set -euo pipefail

if [[ -n "$(git status --porcelain)" ]]; then
  echo "error: working tree must be clean" >&2
  exit 1
fi

git checkout main
git pull --ff-only

create_pr() {
  local branch="$1"
  local title="$2"
  local body="$3"
  git push -u origin "$branch"
  gh pr create --base main --head "$branch" --title "$title" --body "$body" --draft
}

# === PR-A · T0 · CSS-only ===
git checkout -b pr/t0-css-tweak main
mkdir -p web/src/styles
cat > web/src/styles/theme.css <<'CSS'
:root { --brand: #0c5; --bg: #fafafa; }
body { background: var(--bg); color: #111; }
CSS
git add web/src/styles/theme.css
git commit -m "T0: theme color refresh"
create_pr pr/t0-css-tweak "T0 demo · theme color refresh" "Tier-0 CSS-only change. Reviewer should auto-approve."

# === PR-B · T1 · UI component ===
git checkout main
git checkout -b pr/t1-cart-component
mkdir -p web/src/components
cat > web/src/components/Cart.tsx <<'TSX'
export function Cart({ items }: { items: { sku: string; qty: number }[] }) {
  return <ul>{items.map(i => <li key={i.sku}>{i.sku} × {i.qty}</li>)}</ul>;
}
TSX
git add web/src/components/Cart.tsx
git commit -m "T1: cart component skeleton"
create_pr pr/t1-cart-component "T1 demo · cart component" "Tier-1 frontend component. Reviewer should approve with async-human-review tag."

# === PR-C · T2 · backend logic ===
git checkout main
git checkout -b pr/t2-shipping-rate
cat > api/orders/shipping.go <<'GO'
package orders

import "errors"

func ComputeShippingCost(weightG int, zone string) (int64, error) {
    if weightG <= 0 { return 0, errors.New("weight must be positive") }
    rates := map[string]int64{"local": 500, "regional": 1200, "national": 2400}
    rate, ok := rates[zone]
    if !ok { return 0, errors.New("unknown zone") }
    if weightG > 5000 { rate += int64(weightG-5000) / 100 * 50 }
    return rate, nil
}
GO
git add api/orders/shipping.go
git commit -m "T2: shipping cost calculator"
create_pr pr/t2-shipping-rate "T2 demo · shipping rate calculator" "Tier-2 non-PCI backend logic. Reviewer should COMMENT and require 1 human approval."

# === PR-D · T3 · payment rounding (THE MONEY SHOT) ===
git checkout main
git checkout -b pr/t3-payment-rounding
cat > api/payments/tax_calc.go <<'GO'
package payments

import "math"

// CalculateTax returns the cents of tax owed on a subtotal.
func CalculateTax(subtotalCents int64, ratePct float64) int64 {
    return int64(math.Round(float64(subtotalCents) * ratePct / 100.0))
}
GO
git add api/payments/tax_calc.go
git commit -m "T3: tax calculation rounding logic"
create_pr pr/t3-payment-rounding "T3 demo · payment tax rounding" "Tier-3 PCI-scoped change to payment rounding. Reviewer should REQUEST_CHANGES + Slack escalation to #payments-review."

# === PR-E · T4 · migration on payments table (HARD BLOCK) ===
git checkout main
git checkout -b pr/t4-migration-payments
cat > migrations/20260427_add_payments_settlement_batch.sql <<'SQL'
BEGIN;
ALTER TABLE payments ADD COLUMN settlement_batch_id BIGINT NOT NULL DEFAULT 0;
CREATE INDEX idx_payments_settlement_batch_id ON payments(settlement_batch_id);
COMMIT;
SQL
git add migrations/20260427_add_payments_settlement_batch.sql
git commit -m "T4: add settlement_batch_id column to payments"
create_pr pr/t4-migration-payments "T4 demo · payments table migration" "Tier-4 hard block. Reviewer should REQUEST_CHANGES with 'must restructure' verdict + Slack escalation."

# === PR-F · T2→T3 ambiguous (the override case) ===
git checkout main
git checkout -b pr/t2t3-kyc-tax-id
cat >> api/kyc/validator.go <<'GO'

// ExtractTaxIdLast4 returns the last 4 digits of the merchant tax_id.
// PCI-adjacent because tax_id flows into compliance reports.
func ExtractTaxIdLast4(taxID string) string {
    if len(taxID) < 4 { return "" }
    return taxID[len(taxID)-4:]
}
GO
git add api/kyc/validator.go
git commit -m "Extract last 4 of tax_id for compliance reports"
create_pr pr/t2t3-kyc-tax-id "Override demo · KYC tax_id helper" "Boundary case: path matches kyc/ → T3, but the change itself looks innocuous. The reviewer should still classify T3 because the path layer fires."

git checkout main
echo
echo "All 6 demo PRs opened. Watch reviews land:"
echo "  gh run watch"
echo
echo "View PRs:"
gh pr list --head 'pr/' --state open
