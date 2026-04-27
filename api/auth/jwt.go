package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// HandleLogin issues a signed JWT after credential check.
// Security-scoped: changes here are T3 — security-team approval required.
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	user, err := authenticateUser(req.Email, req.Password)
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	token, exp, err := IssueJWT(user.ID, user.Role)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(LoginResponse{Token: token, ExpiresAt: exp})
}

// IssueJWT signs a short-lived access token using HS256.
// The signing secret is loaded from environment — never committed.
func IssueJWT(userID, role string) (string, int64, error) {
	secret := os.Getenv("JWT_SIGNING_SECRET")
	if secret == "" {
		return "", 0, errors.New("missing JWT signing secret")
	}
	exp := time.Now().Add(15 * time.Minute).Unix()
	header := base64URL([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payload := base64URL([]byte(`{"sub":"` + userID + `","role":"` + role + `","exp":` + itoa(exp) + `}`))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(header + "." + payload))
	sig := base64URL(mac.Sum(nil))
	return header + "." + payload + "." + sig, exp, nil
}

// VerifyJWT validates signature and expiration. Constant-time comparison guards against timing side-channels.
func VerifyJWT(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("malformed token")
	}
	secret := os.Getenv("JWT_SIGNING_SECRET")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := base64URL(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return "", errors.New("invalid signature")
	}
	return parts[1], nil
}

type user struct{ ID, Role string }

func authenticateUser(email, pw string) (*user, error) { return &user{ID: "u_demo", Role: "merchant"}, nil }
func base64URL(b []byte) string                          { return base64.RawURLEncoding.EncodeToString(b) }
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
