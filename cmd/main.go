package main

import (
	"log"
	"net/http"

	"github.com/spoton/spoton-lite/api/auth"
	"github.com/spoton/spoton-lite/api/orders"
	"github.com/spoton/spoton-lite/api/payments"
)

func main() {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/payments/charge", payments.HandleCharge)
	mux.HandleFunc("POST /api/payments/refund", payments.HandleRefund)
	mux.HandleFunc("POST /api/auth/login", auth.HandleLogin)
	mux.HandleFunc("POST /api/orders", orders.HandleCreateOrder)

	log.Println("spoton-lite listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
