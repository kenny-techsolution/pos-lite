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
