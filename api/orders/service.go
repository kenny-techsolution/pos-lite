package orders

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
)

type Order struct {
	ID          string    `json:"id"`
	MerchantID  string    `json:"merchant_id"`
	Items       []Item    `json:"items"`
	Subtotal    int64     `json:"subtotal_cents"`
	Tax         int64     `json:"tax_cents"`
	Total       int64     `json:"total_cents"`
	CreatedAt   time.Time `json:"created_at"`
}

type Item struct {
	SKU       string `json:"sku"`
	Quantity  int    `json:"quantity"`
	UnitPrice int64  `json:"unit_price_cents"`
}

// HandleCreateOrder accepts a new order from the POS. T2 — backend business logic, requires 1 human approver.
func HandleCreateOrder(w http.ResponseWriter, r *http.Request) {
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "invalid order", http.StatusBadRequest)
		return
	}
	if err := computeTotals(&order); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	order.ID = "ord_" + time.Now().Format("20060102150405")
	order.CreatedAt = time.Now()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func computeTotals(o *Order) error {
	if len(o.Items) == 0 {
		return errors.New("order must have at least one item")
	}
	var subtotal int64
	for _, item := range o.Items {
		if item.Quantity <= 0 {
			return errors.New("item quantity must be positive")
		}
		subtotal += int64(item.Quantity) * item.UnitPrice
	}
	o.Subtotal = subtotal
	o.Tax = subtotal * 8 / 100 // 8% — should be configurable per merchant
	o.Total = o.Subtotal + o.Tax
	return nil
}
