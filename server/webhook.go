package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cosmin-harangus/go-bpmn-engine/tenant"
	"github.com/go-chi/chi/v5"
)

// WebhookConfig defines how an inbound webhook maps to a BPMN message.
type WebhookConfig struct {
	// MessageName is the BPMN message name to publish (required).
	MessageName string

	// TenantID is the tenant to publish the message for (required).
	// Webhooks bypass the X-Tenant-ID header since external callers won't send it.
	TenantID string

	// CorrelationKeyPath is a dot-separated path into the JSON body to extract
	// the correlation key (e.g. "order.id"). If empty, correlation key is "".
	CorrelationKeyPath string

	// HMACSecret is an optional shared secret for verifying request signatures.
	// When set, the request must include an X-Hub-Signature-256 header containing
	// "sha256=<hex(HMAC-SHA256(secret, body))>". Requests that fail verification
	// are rejected with 401.
	HMACSecret string
}

// RegisterWebhook registers a named webhook endpoint at POST /webhooks/{name}.
// The name must be URL-safe (no slashes).
func (s *Server) RegisterWebhook(name string, cfg WebhookConfig) {
	s.webhooks[name] = cfg
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	cfg, ok := s.webhooks[name]
	if !ok {
		writeError(w, 404, fmt.Sprintf("webhook %q not found", name))
		return
	}

	// Read body (up to 1 MB)
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, 400, "read body: "+err.Error())
		return
	}

	// Verify HMAC signature if configured
	if cfg.HMACSecret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !verifyHMAC(body, sig, cfg.HMACSecret) {
			writeError(w, 401, "invalid signature")
			return
		}
	}

	// Parse body as JSON to extract correlation key and variables
	var payload map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			writeError(w, 400, "body must be valid JSON: "+err.Error())
			return
		}
	}

	correlationKey := extractPath(payload, cfg.CorrelationKeyPath)

	ctx := tenant.WithID(r.Context(), cfg.TenantID)
	if err := s.engine.PublishMessage(ctx, cfg.MessageName, correlationKey, payload); err != nil {
		writeError(w, 422, err.Error())
		return
	}
	w.WriteHeader(204)
}

// verifyHMAC checks that sig == "sha256=<hex(HMAC-SHA256(secret, body))>".
func verifyHMAC(body []byte, sig, secret string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(sig, prefix) {
		return false
	}
	got, err := hex.DecodeString(sig[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := mac.Sum(nil)
	return hmac.Equal(got, expected)
}

// extractPath traverses a dot-separated path in a JSON object and returns the
// value as a string. Returns "" if the path does not resolve to a scalar.
func extractPath(obj map[string]any, path string) string {
	if path == "" || obj == nil {
		return ""
	}
	parts := strings.SplitN(path, ".", 2)
	val, ok := obj[parts[0]]
	if !ok {
		return ""
	}
	if len(parts) == 1 {
		return fmt.Sprintf("%v", val)
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return ""
	}
	return extractPath(nested, parts[1])
}
