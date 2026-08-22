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

// SignatureVerifier checks the authenticity of an inbound webhook request.
// Implementations receive the raw request and body so they can inspect any
// headers or payload fields needed for verification.
type SignatureVerifier interface {
	Verify(r *http.Request, body []byte) bool
}

// HMACVerifier verifies X-Hub-Signature-256 using HMAC-SHA256.
// This is compatible with GitHub, Stripe, and most webhook providers.
//
// Example: HMACVerifier{Secret: os.Getenv("STRIPE_WEBHOOK_SECRET")}
type HMACVerifier struct {
	Secret string
}

func (v HMACVerifier) Verify(r *http.Request, body []byte) bool {
	const prefix = "sha256="
	sig := r.Header.Get("X-Hub-Signature-256")
	if !strings.HasPrefix(sig, prefix) {
		return false
	}
	got, err := hex.DecodeString(sig[len(prefix):])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(v.Secret))
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}

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

	// Verifier authenticates the inbound request (required).
	// All requests must pass verification to be processed; requests with a nil
	// verifier are rejected with 401. Use HMACVerifier as the default implementation.
	Verifier SignatureVerifier
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

	// Read body (up to 1 MB) before verification so the verifier has access to it.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, 400, "read body: "+err.Error())
		return
	}

	// Verify signature when enforcement is enabled (default in production).
	// When enforcement is disabled (e.g. local dev/tests), a nil Verifier skips
	// verification; a non-nil Verifier is still applied.
	if s.enforceWebhookSig {
		if cfg.Verifier == nil || !cfg.Verifier.Verify(r, body) {
			writeError(w, 401, "invalid signature")
			return
		}
	} else if cfg.Verifier != nil && !cfg.Verifier.Verify(r, body) {
		writeError(w, 401, "invalid signature")
		return
	}

	// Parse body as JSON to extract correlation key and variables.
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
