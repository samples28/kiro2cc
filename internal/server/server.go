package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/bestk/kiro2cc/internal/anthropic"
	"github.com/bestk/kiro2cc/internal/config"
	"github.com/bestk/kiro2cc/internal/proxy"
	"github.com/bestk/kiro2cc/internal/token"
	"github.com/bestk/kiro2cc/parser"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server holds the dependencies for the HTTP server.
type Server struct {
	logger *slog.Logger
}

// New creates a new Server.
func New(logger *slog.Logger) *Server {
	return &Server{
		logger: logger,
	}
}

// Start starts the HTTP server on the given port.
func (s *Server) Start(port string) {
	s.logger.Info("Starting Anthropic API proxy server", "port", port)

	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(s.logMiddleware)

	// Endpoints
	r.Post("/v1/messages", s.handleMessages)
	r.Get("/health", s.handleHealth)
	// Add other endpoints here...

	s.logger.Info("Server listening", "address", ":"+port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		s.logger.Error("Failed to start server", "error", err)
		os.Exit(1)
	}
}

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	// Get token
	tok, err := token.ReadToken()
	if err != nil {
		s.logger.Error("Failed to get token", "error", err)
		http.Error(w, "Failed to get token", http.StatusInternalServerError)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("Failed to read request body", "error", err)
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	s.logger.Debug("Anthropic request body", "body", string(body))

	// Parse Anthropic request
	var anthropicReq anthropic.Request
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		s.logger.Error("Failed to parse request body", "error", err)
		http.Error(w, "Failed to parse request body", http.StatusBadRequest)
		return
	}

	// Basic validation
	if anthropicReq.Model == "" {
		http.Error(w, `{"message":"Missing required field: model"}`, http.StatusBadRequest)
		return
	}
	if len(anthropicReq.Messages) == 0 {
		http.Error(w, `{"message":"Missing required field: messages"}`, http.StatusBadRequest)
		return
	}
	if _, ok := proxy.ModelMap[anthropicReq.Model]; !ok {
		available := make([]string, 0, len(proxy.ModelMap))
		for k := range proxy.ModelMap {
			available = append(available, k)
		}
		http.Error(w, fmt.Sprintf(`{"message":"Unknown or unsupported model: %s","availableModels":[%s]}`, anthropicReq.Model, "\""+strings.Join(available, "\",\"")+"\""), http.StatusBadRequest)
		return
	}

	if anthropicReq.Stream {
		s.handleStreamRequest(w, anthropicReq, tok.AccessToken)
	} else {
		s.handleNonStreamRequest(w, anthropicReq, tok.AccessToken)
	}
}

func (s *Server) handleStreamRequest(w http.ResponseWriter, anthropicReq anthropic.Request, accessToken string) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	// Build CodeWhisperer request
	cwReq := proxy.BuildCodeWhispererRequest(anthropicReq)
	cwReqBody, err := json.Marshal(cwReq)
	if err != nil {
		s.sendErrorEvent(w, flusher, "Failed to serialize request", err)
		return
	}

	// Load config to get the region
	cfg, err := config.LoadConfig()
	if err != nil {
		s.sendErrorEvent(w, flusher, "Failed to load configuration", err)
		return
	}

	// Create request
	endpoint := fmt.Sprintf("https://codewhisperer.%s.amazonaws.com/generateAssistantResponse", cfg.Region)
	proxyReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(cwReqBody))
	if err != nil {
		s.sendErrorEvent(w, flusher, "Failed to create proxy request", err)
		return
	}

	proxyReq.Header.Set("Authorization", "Bearer "+accessToken)
	proxyReq.Header.Set("Content-Type", "application/json")
	proxyReq.Header.Set("Accept", "text/event-stream")

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		s.sendErrorEvent(w, flusher, "CodeWhisperer request error", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		s.logger.Error("CodeWhisperer response error", "status_code", resp.StatusCode, "response", string(body))
		s.sendErrorEvent(w, flusher, "error", fmt.Errorf("status code: %d", resp.StatusCode))
		return
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.sendErrorEvent(w, flusher, "error", fmt.Errorf("failed to read CodeWhisperer response"))
		return
	}

	events := parser.ParseEvents(respBody)
	for _, e := range events {
		s.sendSSEEvent(w, flusher, e.Event, e.Data)
		time.Sleep(100 * time.Millisecond) // Simulate streaming
	}
}

func (s *Server) handleNonStreamRequest(w http.ResponseWriter, anthropicReq anthropic.Request, accessToken string) {
	// This function will be refactored to use the new optimizer and cache packages.
	// For now, it will just do the basic proxying.
	cwReq := proxy.BuildCodeWhispererRequest(anthropicReq)
	cwReqBody, err := json.Marshal(cwReq)
	if err != nil {
		s.logger.Error("Failed to serialize request", "error", err)
		http.Error(w, "Failed to serialize request", http.StatusInternalServerError)
		return
	}

	// Load config to get the region
	cfg, err := config.LoadConfig()
	if err != nil {
		s.logger.Error("Failed to load configuration", "error", err)
		http.Error(w, "Failed to load configuration", http.StatusInternalServerError)
		return
	}
	endpoint := fmt.Sprintf("https://codewhisperer.%s.amazonaws.com/generateAssistantResponse", cfg.Region)
	proxyReq, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewBuffer(cwReqBody))
	if err != nil {
		s.logger.Error("Failed to create proxy request", "error", err)
		http.Error(w, "Failed to create proxy request", http.StatusInternalServerError)
		return
	}

	proxyReq.Header.Set("Authorization", "Bearer "+accessToken)
	proxyReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		s.logger.Error("Failed to send request", "error", err)
		http.Error(w, "Failed to send request", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	cwRespBody, err := io.ReadAll(resp.Body)
	if err != nil {
		s.logger.Error("Failed to read response", "error", err)
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	// This part needs to be refactored to properly parse the response and build the Anthropic response.
	// For now, just proxying the raw response.
	w.Header().Set("Content-Type", "application/json")
	w.Write(cwRespBody)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (s *Server) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.logger.Info("Request processed",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"duration", time.Since(start),
			"size", ww.BytesWritten(),
		)
	})
}

func (s *Server) sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType string, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		s.logger.Error("Failed to marshal SSE data", "error", err)
		return
	}

	fmt.Fprintf(w, "event: %s\n", eventType)
	fmt.Fprintf(w, "data: %s\n\n", jsonData)
	flusher.Flush()
}

func (s *Server) sendErrorEvent(w http.ResponseWriter, flusher http.Flusher, message string, err error) {
	s.logger.Error(message, "error", err)
	errorResp := map[string]any{
		"type": "error",
		"error": map[string]any{
			"type":    "overloaded_error",
			"message": message,
		},
	}
	s.sendSSEEvent(w, flusher, "error", errorResp)
}
