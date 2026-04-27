# spoton-lite

Synthetic target repo for the **AI PR Reviewer** demo. Modeled loosely on a SpotOn-shaped fintech POS backend so the reviewer's tier classification has realistic surfaces to chew on.

## Layout

```
api/
  payments/    — T3 PCI-scoped (charge, refund) — senior + domain-owner approval required
  auth/        — T3 JWT issuance + verification — security-team approval required
  kyc/         — T3 PCI-adjacent merchant onboarding — compliance-team approval
  crypto/      — T3 key management
  orders/      — T2 business logic (non-PCI backend) — 1 human approver
  notifications/ — T2 outbound SMS / email
  users/       — T2 profile management
  inventory/   — T2 stock management
  menu/        — T2 menu items
  reports/     — T2 reporting exports
migrations/    — T4 (when touching payment-table schema) — hard block
.github/
  CODEOWNERS         — drives reviewer's path-based signal layer
  workflows/         — ai-review.yml runs the reviewer on every PR
```

## What the reviewer does on this repo

For each PR opened or updated:

1. Reads the diff and changed file paths
2. Runs the 4-layer risk-signal stack (path · diff · AST via tree-sitter · LLM classifier when ambiguous)
3. Routes to specialist agents (security, PCI, architecture)
4. Aggregator emits a tier (T0–T4) + risk report
5. **Posts a GitHub Review** (APPROVE / COMMENT / REQUEST_CHANGES) with line-level comments
6. **Sends Slack escalation** for T3 / T4 to `#payments-review` or `#security`
7. Writes a structured event to `artifacts/events.jsonl` (drives the metrics dashboard)

## Why "AI is not the gatekeeper"

Branch protection rules (described in `.github/CODEOWNERS` and configured in repo settings) require **N approvals from listed CODEOWNERS** for any PR touching payment / auth / KYC paths. The reviewer's `REQUEST_CHANGES` is one signal feeding into a system engineered so that — even if the AI hallucinated an APPROVE on a T3 path — GitHub itself blocks the merge until a human in `@spoton/payments-team` and `@spoton/security-leads` approves.

The AI reviewer is a risk report, not the enforcer. GitHub primitives are.

## Running locally (smoke)

```
cd cmd && go run main.go
```

Listens on `:8080`. Endpoints exist for shape only — they hand off to mocked processors.
