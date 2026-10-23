package http

import (
	"context"
	"encoding/json"
	"errors"
	"example.com/financial-service/internal/application"
	"example.com/financial-service/internal/domain"
	"example.com/financial-service/internal/infrastructure/authentication"
	"example.com/financial-service/internal/infrastructure/messaging"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type Handler struct {
	Service                                              *application.Service
	Auth                                                 *authentication.Validator
	Queues                                               *messaging.Client
	processed, rejected, replays, conflicts, divergences atomic.Uint64
}

func NewHandler(service *application.Service, auth *authentication.Validator, queues *messaging.Client) *Handler {
	return &Handler{Service: service, Auth: auth, Queues: queues}
}
func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	err := json.NewEncoder(writer).Encode(value)
	if err != nil {
		slog.Warn("response write failed", "error", err)
	}
}
func decode(writer http.ResponseWriter, request *http.Request, value any) error {
	request.Body = http.MaxBytesReader(writer, request.Body, 65536)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(value)
	if err != nil {
		return domain.ErrInvalidOperation
	}
	if decoder.Decode(new(any)) != io.EOF {
		return domain.ErrInvalidOperation
	}
	return nil
}
func (h *Handler) authorize(writer http.ResponseWriter, request *http.Request, access string) (authentication.Claims, bool) {
	header := request.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		writeJSON(writer, 401, map[string]string{"error": "UNAUTHENTICATED"})
		return authentication.Claims{}, false
	}
	claims, err := h.Auth.Verify(request.Context(), strings.TrimPrefix(header, "Bearer "))
	if err != nil {
		writeJSON(writer, 401, map[string]string{"error": "UNAUTHENTICATED"})
		return claims, false
	}
	if claims.Access != access {
		writeJSON(writer, 403, map[string]string{"error": "FORBIDDEN"})
		return claims, false
	}
	return claims, true
}
func (h *Handler) failure(writer http.ResponseWriter, err error) {
	status := 503
	code := "TEMPORARILY_UNAVAILABLE"
	switch {
	case errors.Is(err, domain.ErrConflict):
		status = 409
		code = "CONFLICT"
		h.conflicts.Add(1)
	case errors.Is(err, domain.ErrNotFound):
		status = 404
		code = "NOT_FOUND"
	case errors.Is(err, domain.ErrForbidden):
		status = 403
		code = "FORBIDDEN"
	case errors.Is(err, domain.ErrInvalidOperation), errors.Is(err, domain.ErrInvalidAmount), errors.Is(err, domain.ErrInvalidCurrency), errors.Is(err, domain.ErrAmountOutOfRange), errors.Is(err, domain.ErrCurrencyMismatch), errors.Is(err, domain.ErrInvalidWalletID), errors.Is(err, domain.ErrInvalidPlayerID), errors.Is(err, domain.ErrNegativeInitialBalance):
		status = 400
		code = "INVALID_INPUT"
	}
	if status == 503 {
		slog.Error("request infrastructure failure", "error", err)
		writer.Header().Set("Retry-After", "2")
	}
	writeJSON(writer, status, map[string]string{"error": code})
}
func (h *Handler) CreateWallet(writer http.ResponseWriter, request *http.Request) {
	_, ok := h.authorize(writer, request, "internal")
	if !ok {
		return
	}
	var input struct {
		PlayerID       string            `json:"playerId"`
		InitialBalance domain.MoneyInput `json:"initialBalance"`
	}
	err := decode(writer, request, &input)
	if err != nil {
		h.failure(writer, err)
		return
	}
	result, err := h.Service.CreateWallet(request.Context(), input.PlayerID, input.InitialBalance)
	if err != nil {
		h.failure(writer, err)
		return
	}
	writeJSON(writer, 201, result)
}
func (h *Handler) Wallet(writer http.ResponseWriter, request *http.Request) {
	_, ok := h.authorize(writer, request, "internal")
	if !ok {
		return
	}
	result, err := h.Service.Wallet(request.Context(), request.PathValue("walletID"))
	if err != nil {
		h.failure(writer, err)
		return
	}
	writeJSON(writer, 200, result)
}
func (h *Handler) Process(writer http.ResponseWriter, request *http.Request) {
	claims, ok := h.authorize(writer, request, "provider")
	if !ok {
		return
	}
	var operation domain.Operation
	err := decode(writer, request, &operation)
	if err != nil {
		h.failure(writer, err)
		return
	}
	if operation.ProviderID != claims.ProviderID {
		h.failure(writer, domain.ErrForbidden)
		return
	}
	operation.IdempotencyKey = request.Header.Get("Idempotency-Key")
	correlation := request.Header.Get("X-Correlation-ID")
	if len(correlation) > 256 {
		h.failure(writer, domain.ErrInvalidOperation)
		return
	}
	if correlation == "" {
		correlation, err = domain.NewID()
		if err != nil {
			h.failure(writer, err)
			return
		}
	}
	writer.Header().Set("X-Correlation-ID", correlation)
	result, err := h.Service.Process(request.Context(), operation, correlation, nil)
	if err != nil {
		h.failure(writer, err)
		return
	}
	status := 200
	if result.Status == "REJECTED" {
		status = 422
		h.rejected.Add(1)
	} else if result.Status == "PENDING_REFERENCE" || result.Status == "PENDING" {
		status = 202
	} else {
		h.processed.Add(1)
	}
	if result.IdempotentReplay {
		h.replays.Add(1)
	}
	slog.Info("transaction result", "correlationId", correlation, "transactionId", result.TransactionID, "walletId", operation.WalletID, "providerId", claims.ProviderID, "status", result.Status)
	writeJSON(writer, status, result)
}
func (h *Handler) Transaction(writer http.ResponseWriter, request *http.Request) {
	claims, ok := h.authorize(writer, request, "provider")
	if !ok {
		return
	}
	provider := request.PathValue("providerID")
	if provider != "" && provider != claims.ProviderID {
		h.failure(writer, domain.ErrForbidden)
		return
	}
	result, err := h.Service.Transaction(request.Context(), claims.ProviderID, request.PathValue("transactionID"), request.PathValue("externalID"))
	if err != nil {
		h.failure(writer, err)
		return
	}
	writeJSON(writer, 200, result)
}
func (h *Handler) Ledger(writer http.ResponseWriter, request *http.Request) {
	_, ok := h.authorize(writer, request, "internal")
	if !ok {
		return
	}
	limit := 50
	var err error
	if request.URL.Query().Get("limit") != "" {
		limit, err = strconv.Atoi(request.URL.Query().Get("limit"))
		if err != nil {
			h.failure(writer, domain.ErrInvalidOperation)
			return
		}
	}
	result, err := h.Service.Ledger(request.Context(), request.PathValue("walletID"), request.URL.Query().Get("cursor"), limit)
	if err != nil {
		h.failure(writer, err)
		return
	}
	writeJSON(writer, 200, result)
}
func (h *Handler) Reconcile(writer http.ResponseWriter, request *http.Request) {
	_, ok := h.authorize(writer, request, "internal")
	if !ok {
		return
	}
	id := request.PathValue("walletID")
	result, err := h.Service.Reconcile(request.Context(), id)
	if err != nil {
		h.failure(writer, err)
		return
	}
	if data, ok := result.(map[string]any); ok && data["consistent"] == false {
		h.divergences.Add(1)
		slog.Error("reconciliation mismatch", "walletId", id)
	}
	writeJSON(writer, 200, result)
}
func (h *Handler) Live(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, 200, map[string]string{"status": "running"})
}
func (h *Handler) Ready(writer http.ResponseWriter, request *http.Request) {
	err := h.Service.Ready(request.Context())
	if err == nil {
		err = h.Queues.Ready(request.Context())
	}
	if err != nil {
		writeJSON(writer, 503, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(writer, 200, map[string]string{"status": "ready"})
}
func (h *Handler) Metrics(writer http.ResponseWriter, request *http.Request) {
	_, ok := h.authorize(writer, request, "internal")
	if !ok {
		return
	}
	writer.Header().Set("Content-Type", "text/plain; version=0.0.4")
	fmt.Fprintf(writer, "wager_http_processed_total %d\nwager_http_rejected_total %d\nwager_http_replays_total %d\nwager_http_conflicts_total %d\nreconciliation_mismatches_total %d\n", h.processed.Load(), h.rejected.Load(), h.replays.Load(), h.conflicts.Load(), h.divergences.Load())
}

type timeoutHandler struct{ next http.Handler }

func (handler timeoutHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	ctx, cancel := context.WithTimeout(request.Context(), 15*time.Second)
	defer cancel()
	handler.next.ServeHTTP(writer, request.WithContext(ctx))
}
func (h *Handler) Router() http.Handler {
	router := http.NewServeMux()
	router.HandleFunc("GET /health/live", h.Live)
	router.HandleFunc("GET /health/ready", h.Ready)
	router.HandleFunc("GET /metrics", h.Metrics)
	router.HandleFunc("POST /wallets", h.CreateWallet)
	router.HandleFunc("GET /wallets/{walletID}", h.Wallet)
	router.HandleFunc("GET /wallets/{walletID}/ledger", h.Ledger)
	router.HandleFunc("POST /wallets/{walletID}/reconciliation", h.Reconcile)
	router.HandleFunc("POST /wagering/transactions", h.Process)
	router.HandleFunc("GET /wagering/transactions/{transactionID}", h.Transaction)
	router.HandleFunc("GET /providers/{providerID}/wagering/transactions/{externalID}", h.Transaction)
	return timeoutHandler{next: router}
}
