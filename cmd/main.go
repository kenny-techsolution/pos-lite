package main

import (
	"log"
	"net/http"

	"github.com/kenny-techsolution/pos-lite/api/auth"
	"github.com/kenny-techsolution/pos-lite/api/orders"
	"github.com/kenny-techsolution/pos-lite/api/payments"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/payments/charge", payments.HandleCharge)
	mux.HandleFunc("POST /api/payments/refund", payments.HandleRefund)
	mux.HandleFunc("POST /api/auth/login", auth.HandleLogin)
	mux.HandleFunc("POST /api/orders", orders.HandleCreateOrder)

	log.Println("pos-lite listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
