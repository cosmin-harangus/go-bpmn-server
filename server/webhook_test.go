package server_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cosmin-harangus/go-bpmn-engine/store"
	bpmnserver "github.com/cosmin-harangus/go-bpmn-server/server"
)

// mockEngine satisfies bpmnserver.Engine. Only PublishMessage is used by webhook tests.
type mockEngine struct {
	publishedMessages []publishedMsg
	publishErr        error
}

type publishedMsg struct {
	ctx            context.Context
	messageName    string
	correlationKey string
	vars           map[string]any
}

func (m *mockEngine) DeployProcess(ctx context.Context, bpmnXML []byte) (interface{}, error) {
	return nil, nil
}
func (m *mockEngine) CreateInstance(ctx context.Context, processKey string, vars map[string]any) (string, error) {
	return "", nil
}
func (m *mockEngine) RunInstance(ctx context.Context, instanceID string) error { return nil }
func (m *mockEngine) ListInstances(ctx context.Context, q store.InstanceQuery) ([]store.ProcessInstance, int, error) {
	return nil, 0, nil
}
func (m *mockEngine) GetInstanceDetail(ctx context.Context, id string) (*store.InstanceDetail, error) {
	return nil, nil
}
func (m *mockEngine) CancelInstance(ctx context.Context, id string) error  { return nil }
func (m *mockEngine) SuspendInstance(ctx context.Context, id string) error { return nil }
func (m *mockEngine) ResumeInstance(ctx context.Context, id string) error  { return nil }
func (m *mockEngine) CompleteJob(ctx context.Context, id string, vars map[string]any) error {
	return nil
}
func (m *mockEngine) FailJob(ctx context.Context, id string, retries int, msg string) error {
	return nil
}
func (m *mockEngine) CompleteUserTask(ctx context.Context, id string, vars map[string]any) error {
	return nil
}
func (m *mockEngine) ListUserTasks(ctx context.Context, q store.UserTaskQuery) ([]store.UserTask, int, error) {
	return nil, 0, nil
}
func (m *mockEngine) ClaimTask(ctx context.Context, id, user string) error { return nil }
func (m *mockEngine) UnclaimTask(ctx context.Context, id string) error     { return nil }
func (m *mockEngine) ListIncidents(ctx context.Context, q store.IncidentQuery) ([]store.Incident, int, error) {
	return nil, 0, nil
}
func (m *mockEngine) ResolveIncident(ctx context.Context, id string, retries int, vars map[string]any) error {
	return nil
}
func (m *mockEngine) PublishMessage(ctx context.Context, name, key string, vars map[string]any) error {
	if m.publishErr != nil {
		return m.publishErr
	}
	m.publishedMessages = append(m.publishedMessages, publishedMsg{ctx, name, key, vars})
	return nil
}

func setupWebhookServer(t *testing.T) (*bpmnserver.Server, *mockEngine) {
	t.Helper()
	eng := &mockEngine{}
	srv := bpmnserver.New(eng, ":0")
	return srv, eng
}

func signHMAC(body []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// alwaysAllow is a SignatureVerifier that accepts every request — useful in tests
// that focus on routing/payload behaviour, not authentication.
type alwaysAllow struct{}

func (alwaysAllow) Verify(_ *http.Request, _ []byte) bool { return true }

// headerVerifier checks a custom header value — demonstrates pluggable verifiers.
type headerVerifier struct{ header, value string }

func (v headerVerifier) Verify(r *http.Request, _ []byte) bool {
	return r.Header.Get(v.header) == v.value
}

func TestWebhook_NotFound(t *testing.T) {
	srv, _ := setupWebhookServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/webhooks/unknown", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestWebhook_NilVerifierRejected(t *testing.T) {
	srv, _ := setupWebhookServer(t)
	srv.RegisterWebhook("unsafe", bpmnserver.WebhookConfig{
		MessageName: "Event",
		TenantID:    "acme",
		Verifier:    nil, // no verifier — must be rejected
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/webhooks/unsafe", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("expected 401 for nil verifier, got %d", resp.StatusCode)
	}
}

func TestWebhook_PublishesMessage(t *testing.T) {
	srv, eng := setupWebhookServer(t)
	srv.RegisterWebhook("stripe", bpmnserver.WebhookConfig{
		MessageName:        "PaymentReceived",
		TenantID:           "acme",
		CorrelationKeyPath: "data.object.id",
		Verifier:           alwaysAllow{},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := []byte(`{"data":{"object":{"id":"ch_123","amount":9900}}}`)
	resp, err := http.Post(ts.URL+"/webhooks/stripe", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if len(eng.publishedMessages) != 1 {
		t.Fatalf("expected 1 published message, got %d", len(eng.publishedMessages))
	}
	msg := eng.publishedMessages[0]
	if msg.messageName != "PaymentReceived" {
		t.Errorf("messageName: got %q, want %q", msg.messageName, "PaymentReceived")
	}
	if msg.correlationKey != "ch_123" {
		t.Errorf("correlationKey: got %q, want %q", msg.correlationKey, "ch_123")
	}
}

func TestWebhook_NoCorrelationKey(t *testing.T) {
	srv, eng := setupWebhookServer(t)
	srv.RegisterWebhook("notify", bpmnserver.WebhookConfig{
		MessageName: "GenericEvent",
		TenantID:    "acme",
		Verifier:    alwaysAllow{},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := []byte(`{"event":"ping"}`)
	resp, err := http.Post(ts.URL+"/webhooks/notify", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if eng.publishedMessages[0].correlationKey != "" {
		t.Errorf("expected empty correlation key, got %q", eng.publishedMessages[0].correlationKey)
	}
}

func TestWebhook_HMACValid(t *testing.T) {
	srv, eng := setupWebhookServer(t)
	secret := "supersecret"
	srv.RegisterWebhook("github", bpmnserver.WebhookConfig{
		MessageName:        "PushEvent",
		TenantID:           "acme",
		CorrelationKeyPath: "repository.full_name",
		Verifier:           bpmnserver.HMACVerifier{Secret: secret},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := []byte(`{"repository":{"full_name":"acme/myrepo"}}`)
	req, _ := http.NewRequest("POST", ts.URL+"/webhooks/github", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", signHMAC(body, secret))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if eng.publishedMessages[0].correlationKey != "acme/myrepo" {
		t.Errorf("correlationKey: got %q", eng.publishedMessages[0].correlationKey)
	}
}

func TestWebhook_HMACInvalid(t *testing.T) {
	srv, _ := setupWebhookServer(t)
	srv.RegisterWebhook("github", bpmnserver.WebhookConfig{
		MessageName: "PushEvent",
		TenantID:    "acme",
		Verifier:    bpmnserver.HMACVerifier{Secret: "supersecret"},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := []byte(`{"repository":{"full_name":"acme/myrepo"}}`)
	req, _ := http.NewRequest("POST", ts.URL+"/webhooks/github", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Hub-Signature-256", "sha256=deadbeef")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 401 {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWebhook_CustomVerifier(t *testing.T) {
	srv, eng := setupWebhookServer(t)
	srv.RegisterWebhook("internal", bpmnserver.WebhookConfig{
		MessageName: "InternalEvent",
		TenantID:    "acme",
		Verifier:    headerVerifier{header: "X-Api-Key", value: "secret-key"},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := []byte(`{"type":"ping"}`)

	t.Run("valid key", func(t *testing.T) {
		req, _ := http.NewRequest("POST", ts.URL+"/webhooks/internal", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", "secret-key")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 204 {
			t.Errorf("expected 204, got %d", resp.StatusCode)
		}
		if len(eng.publishedMessages) != 1 {
			t.Errorf("expected 1 message, got %d", len(eng.publishedMessages))
		}
	})

	t.Run("wrong key", func(t *testing.T) {
		req, _ := http.NewRequest("POST", ts.URL+"/webhooks/internal", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Api-Key", "wrong")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != 401 {
			t.Errorf("expected 401, got %d", resp.StatusCode)
		}
	})
}

func TestWebhook_InvalidJSON(t *testing.T) {
	srv, _ := setupWebhookServer(t)
	srv.RegisterWebhook("test", bpmnserver.WebhookConfig{
		MessageName: "Event",
		TenantID:    "acme",
		Verifier:    alwaysAllow{},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/webhooks/test", "application/json", bytes.NewReader([]byte("not json")))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestWebhook_EngineError(t *testing.T) {
	srv, eng := setupWebhookServer(t)
	eng.publishErr = fmt.Errorf("no subscription found")
	srv.RegisterWebhook("test", bpmnserver.WebhookConfig{
		MessageName: "Event",
		TenantID:    "acme",
		Verifier:    alwaysAllow{},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := []byte(`{}`)
	resp, err := http.Post(ts.URL+"/webhooks/test", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 422 {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
	var errBody map[string]string
	json.NewDecoder(resp.Body).Decode(&errBody)
	if errBody["error"] == "" {
		t.Error("expected error message in response")
	}
}

func TestWebhook_NestedCorrelationPath(t *testing.T) {
	srv, eng := setupWebhookServer(t)
	srv.RegisterWebhook("deep", bpmnserver.WebhookConfig{
		MessageName:        "DeepEvent",
		TenantID:           "acme",
		CorrelationKeyPath: "a.b.c",
		Verifier:           alwaysAllow{},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := []byte(`{"a":{"b":{"c":"found-me"}}}`)
	resp, err := http.Post(ts.URL+"/webhooks/deep", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	if eng.publishedMessages[0].correlationKey != "found-me" {
		t.Errorf("correlationKey: got %q", eng.publishedMessages[0].correlationKey)
	}
}
