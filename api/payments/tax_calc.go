package payments

import "math"

// CalculateTax returns the cents of tax owed on a subtotal.
func CalculateTax(subtotalCents int64, ratePct float64) int64 {
    return int64(math.Round(float64(subtotalCents) * ratePct / 100.0))
}
