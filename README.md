# pos-lite

Synthetic target repo for the **AI PR Reviewer** demo. Modeled as a generic fintech-shaped POS backend so the reviewer's tier classification has realistic surfaces to evaluate.

## Layout

```
api/
  payments/      — T3 PCI-scoped (charge, refund) · senior + domain-owner approval required
  auth/          — T3 JWT issuance + verification · security-team approval required
  kyc/           — T3 PCI-adjacent merchant onboarding · compliance-team approval
  crypto/        — T3 key management
  orders/        — T2 business logic (non-PCI backend) · 1 human approver
  notifications/ — T2 outbound SMS / email
  users/         — T2 profile management
  inventory/     — T2 stock management
  menu/          — T2 menu items
  reports/       — T2 reporting exports
migrations/      — T4 when touching payment-table schema · hard block
.github/
  CODEOWNERS         — drives merge gating + audit-team auto-watch
  workflows/         — ai-review.yml runs the AI PR Reviewer on every PR
```

## What the reviewer does on this repo

For each PR opened or updated:

1. Reads the diff and changed file paths
2. Runs the 4-layer risk-signal stack (path · diff · semantic · LLM classifier when ambiguous)
3. Routes to specialist agents (security, PCI, architecture)
4. Aggregator emits a tier (T0–T4) + risk report
5. **Posts a GitHub Review** (APPROVE / COMMENT / REQUEST_CHANGES) with line-level comments
6. **Sends Slack escalation** for T3 / T4 to `#payments-review` or `#security`
7. Writes a structured event to `artifacts/events.jsonl` (the metrics-dashboard substrate)

## Why "AI is not the gatekeeper"

Branch protection rules (described in `.github/CODEOWNERS` and configured in repo settings) require approvals from listed CODEOWNERS for any PR touching payment / auth / KYC paths. The reviewer's `REQUEST_CHANGES` is one signal feeding into a system engineered so that — even if the AI hallucinated an APPROVE on a T3 path — **GitHub itself blocks the merge** until a human in the payments / security CODEOWNERS approves.

The AI reviewer is a risk report. GitHub primitives are the enforcer.

## Running locally (smoke)

```
cd cmd && go run main.go
```

Listens on `:8080`. Endpoints exist for shape only — they hand off to mocked processors.

## Linked repos

- [`ai-pr-reviewer`](https://github.com/kenny-techsolution/ai-pr-reviewer) — the reviewer itself, checked out and run by `ai-review.yml` on every PR.
