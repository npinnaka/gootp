package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"time"

	redis "github.com/redis/go-redis/v9"
)

// Server holds dependencies for HTTP handlers.
type Server struct {
	redis *redis.Client
}

// New creates a new Server with the provided Redis client.
func New(rdb *redis.Client) *Server {
	return &Server{redis: rdb}
}

// Request/response models

type GenerateRequest struct {
	UserID string `json:"userId"`
}

type GenerateResponse struct {
	UserID    string `json:"userId"`
	OTP       string `json:"otp"`
	ExpiresIn int    `json:"expiresInSeconds"`
}

type ValidateRequest struct {
	UserID string `json:"userId"`
	OTP    string `json:"otp"`
}

type ValidateResponse struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message,omitempty"`
}

const (
	otpTTL       = 5 * time.Minute
	otpKeyPrefix = "otp:"
)

// HandleGenerate generates a new OTP for the given userId.
func (s *Server) HandleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req GenerateRequest
	if err := decodeJSON(r, &req); err != nil || req.UserID == "" {
		http.Error(w, "userId is required (provide JSON body)", http.StatusBadRequest)
		return
	}

	otp, err := generateOTP(6)
	if err != nil {
		http.Error(w, "failed to generate otp", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	key := otpKeyPrefix + req.UserID
	if err := s.redis.Set(ctx, key, otp, otpTTL).Err(); err != nil {
		http.Error(w, "failed to store otp", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, GenerateResponse{UserID: req.UserID, OTP: otp, ExpiresIn: int(otpTTL.Seconds())})
}

// HandleValidate validates an OTP for the given userId.
func (s *Server) HandleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ValidateRequest
	if err := decodeJSON(r, &req); err != nil || req.UserID == "" || req.OTP == "" {
		http.Error(w, "userId and otp are required (provide JSON body)", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	key := otpKeyPrefix + req.UserID
	stored, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			writeJSON(w, http.StatusOK, ValidateResponse{Valid: false, Message: "otp not found or expired"})
			return
		}
		http.Error(w, "failed to read otp", http.StatusInternalServerError)
		return
	}

	if subtleConstTimeCompare(stored, req.OTP) {
		// delete the key to prevent replay
		_, _ = s.redis.Del(ctx, key).Result()
		writeJSON(w, http.StatusOK, ValidateResponse{Valid: true, Message: "otp valid"})
		return
	}
	writeJSON(w, http.StatusOK, ValidateResponse{Valid: false, Message: "invalid otp"})
}

func generateOTP(digits int) (string, error) {
	max := int64(1)
	for i := 0; i < digits; i++ {
		max *= 10
	}
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return "", err
	}
	format := fmt.Sprintf("%%0%dd", digits)
	return fmt.Sprintf(format, n.Int64()), nil
}

func subtleConstTimeCompare(a, b string) bool {
	// simple constant-time compare to avoid timing leaks
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

func decodeJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
