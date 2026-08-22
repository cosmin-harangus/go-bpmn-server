//go:build e2e

package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/cosmin-harangus/go-bpmn-engine/engine"
	"github.com/cosmin-harangus/go-bpmn-engine/store"
	pgstore "github.com/cosmin-harangus/go-bpmn-engine/store/pg"
	"github.com/cosmin-harangus/go-bpmn-engine/tenant"
	bpmnserver "github.com/cosmin-harangus/go-bpmn-server/server"
	"github.com/cosmin-harangus/go-bpmn-server/server/auth/noauth"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// engineAdapter bridges *engine.Engine to bpmnserver.Engine.
// DeployProcess returns (*store.ProcessDef, error) on the concrete type,
// but the interface requires (interface{}, error).
type engineAdapter struct {
	eng *engine.Engine
}

func (a *engineAdapter) DeployProcess(ctx context.Context, bpmnXML []byte) (interface{}, error) {
	return a.eng.DeployProcess(ctx, bpmnXML)
}
func (a *engineAdapter) CreateInstance(ctx context.Context, processKey string, vars map[string]any) (string, error) {
	return a.eng.CreateInstance(ctx, processKey, vars)
}
func (a *engineAdapter) RunInstance(ctx context.Context, instanceID string) error {
	return a.eng.RunInstance(ctx, instanceID)
}
func (a *engineAdapter) ListInstances(ctx context.Context, query store.InstanceQuery) ([]store.ProcessInstance, int, error) {
	return a.eng.ListInstances(ctx, query)
}
func (a *engineAdapter) GetInstanceDetail(ctx context.Context, instanceID string) (*store.InstanceDetail, error) {
	return a.eng.GetInstanceDetail(ctx, instanceID)
}
func (a *engineAdapter) CancelInstance(ctx context.Context, instanceID string) error {
	return a.eng.CancelInstance(ctx, instanceID)
}
func (a *engineAdapter) SuspendInstance(ctx context.Context, instanceID string) error {
	return a.eng.SuspendInstance(ctx, instanceID)
}
func (a *engineAdapter) ResumeInstance(ctx context.Context, instanceID string) error {
	return a.eng.ResumeInstance(ctx, instanceID)
}
func (a *engineAdapter) CompleteJob(ctx context.Context, jobID string, vars map[string]any) error {
	return a.eng.CompleteJob(ctx, jobID, vars)
}
func (a *engineAdapter) FailJob(ctx context.Context, jobID string, retries int, errMsg string) error {
	return a.eng.FailJob(ctx, jobID, retries, errMsg)
}
func (a *engineAdapter) CompleteUserTask(ctx context.Context, taskID string, vars map[string]any) error {
	return a.eng.CompleteUserTask(ctx, taskID, vars)
}
func (a *engineAdapter) ListUserTasks(ctx context.Context, query store.UserTaskQuery) ([]store.UserTask, int, error) {
	return a.eng.ListUserTasks(ctx, query)
}
func (a *engineAdapter) ClaimTask(ctx context.Context, taskID, userID string) error {
	return a.eng.ClaimTask(ctx, taskID, userID)
}
func (a *engineAdapter) UnclaimTask(ctx context.Context, taskID string) error {
	return a.eng.UnclaimTask(ctx, taskID)
}
func (a *engineAdapter) ListIncidents(ctx context.Context, query store.IncidentQuery) ([]store.Incident, int, error) {
	return a.eng.ListIncidents(ctx, query)
}
func (a *engineAdapter) ResolveIncident(ctx context.Context, incidentID string, retries int, vars map[string]any) error {
	return a.eng.ResolveIncident(ctx, incidentID, retries, vars)
}
func (a *engineAdapter) PublishMessage(ctx context.Context, messageName, correlationKey string, vars map[string]any) error {
	return a.eng.PublishMessage(ctx, messageName, correlationKey, vars)
}

var sharedConnStr string

func TestMain(m *testing.M) {
	os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")

	ctx := context.Background()
	c, err := postgres.Run(ctx, "postgres:16-alpine",
		postgres.WithDatabase("bpmn"),
		postgres.WithUsername("bpmn"),
		postgres.WithPassword("bpmn"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		os.Exit(1)
	}

	sharedConnStr, _ = c.ConnectionString(ctx, "sslmode=disable")
	if err := pgstore.RunMigrations(ctx, sharedConnStr); err != nil {
		fmt.Fprintf(os.Stderr, "run migrations: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	c.Terminate(ctx)
	os.Exit(code)
}

type testEnv struct {
	ts  *httptest.Server
	eng *engine.Engine
	s   store.Store
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	s, err := pgstore.New(sharedConnStr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	eng, err := engine.New(s)
	if err != nil {
		t.Fatal(err)
	}

	srv := bpmnserver.New(&engineAdapter{eng: eng}, ":0", bpmnserver.WithAuthenticator(&noauth.NoAuth{}))
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	return &testEnv{ts: ts, eng: eng, s: s}
}

func (e *testEnv) doRequest(t *testing.T, method, path string, body any, tenantID string) *http.Response {
	t.Helper()
	var reqBody []byte
	if body != nil {
		switch v := body.(type) {
		case []byte:
			reqBody = v
		default:
			var err error
			reqBody, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	req, err := http.NewRequest(method, e.ts.URL+path, bytes.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	if tenantID != "" {
		req.Header.Set("X-Tenant-ID", tenantID)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func readBPMN(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../testdata/" + name)
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return b
}

func TestAPI_TenantHeaderRequired(t *testing.T) {
	env := setup(t)

	resp := env.doRequest(t, "POST", "/processes", []byte("<definitions/>"), "")
	defer resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("expected 400 without tenant header, got %d", resp.StatusCode)
	}

	var errResp map[string]string
	json.NewDecoder(resp.Body).Decode(&errResp)
	if errResp["error"] != "X-Tenant-ID header is required" {
		t.Errorf("unexpected error: %s", errResp["error"])
	}
}

func TestAPI_DeployProcess(t *testing.T) {
	env := setup(t)
	tid := "deploy-test"

	t.Run("valid BPMN", func(t *testing.T) {
		bpmn := readBPMN(t, "simple-sequence.bpmn")
		resp := env.doRequest(t, "POST", "/processes", bpmn, tid)
		defer resp.Body.Close()

		if resp.StatusCode != 201 {
			t.Fatalf("expected 201, got %d", resp.StatusCode)
		}
		var result map[string]any
		json.NewDecoder(resp.Body).Decode(&result)
		if result["ID"] == nil && result["id"] == nil {
			t.Errorf("expected response with ID field, got %v", result)
		}
	})

	t.Run("invalid BPMN", func(t *testing.T) {
		resp := env.doRequest(t, "POST", "/processes", []byte("not xml"), tid)
		defer resp.Body.Close()

		if resp.StatusCode != 422 {
			t.Errorf("expected 422 for invalid BPMN, got %d", resp.StatusCode)
		}
	})
}

func TestAPI_CreateAndRunInstance(t *testing.T) {
	env := setup(t)
	tid := "instance-test"
	ctx := tenant.WithID(context.Background(), tid)

	bpmn := readBPMN(t, "simple-sequence.bpmn")
	if _, err := env.eng.DeployProcess(ctx, bpmn); err != nil {
		t.Fatalf("DeployProcess: %v", err)
	}

	resp := env.doRequest(t, "POST", "/instances", map[string]any{
		"process_key": "simple-sequence",
		"variables":   map[string]any{"x": 1},
	}, tid)
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)
	id := result["id"]
	if id == "" {
		t.Fatal("expected instance id in response")
	}

	inst, err := env.s.GetInstance(ctx, id)
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if inst.State != store.StateActive {
		t.Errorf("expected active, got %q", inst.State)
	}
}

func TestAPI_CompleteJob(t *testing.T) {
	env := setup(t)
	tid := "job-test"
	ctx := tenant.WithID(context.Background(), tid)

	bpmn := readBPMN(t, "service-task.bpmn")
	if _, err := env.eng.DeployProcess(ctx, bpmn); err != nil {
		t.Fatalf("DeployProcess: %v", err)
	}

	id, err := env.eng.CreateInstance(ctx, "service-task-process", map[string]any{"input": "hello"})
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	if err := env.eng.RunInstance(ctx, id); err != nil {
		t.Fatalf("RunInstance: %v", err)
	}

	jobs, err := env.s.PollJobs(ctx, store.PollJobsParams{
		JobType: "call-api", WorkerID: "test", MaxJobs: 10, TenantID: tid,
	})
	if err != nil || len(jobs) == 0 {
		t.Fatalf("PollJobs: err=%v, got %d jobs", err, len(jobs))
	}
	var jobID string
	for _, j := range jobs {
		if j.ProcessInstanceID == id {
			jobID = j.ID
			break
		}
	}
	if jobID == "" {
		t.Fatal("no job found for this instance")
	}

	resp := env.doRequest(t, "POST", "/jobs/"+jobID+"/complete", map[string]any{
		"variables": map[string]any{"output": "world"},
	}, tid)
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	inst, err := env.s.GetInstance(ctx, id)
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if inst.State != store.StateCompleted {
		t.Errorf("expected completed, got %q", inst.State)
	}
}

func TestAPI_FailJob(t *testing.T) {
	env := setup(t)
	tid := "fail-job-test"
	ctx := tenant.WithID(context.Background(), tid)

	bpmn := readBPMN(t, "service-task.bpmn")
	if _, err := env.eng.DeployProcess(ctx, bpmn); err != nil {
		t.Fatalf("DeployProcess: %v", err)
	}
	id, err := env.eng.CreateInstance(ctx, "service-task-process", map[string]any{"input": "x"})
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	if err := env.eng.RunInstance(ctx, id); err != nil {
		t.Fatalf("RunInstance: %v", err)
	}

	jobs, err := env.s.PollJobs(ctx, store.PollJobsParams{
		JobType: "call-api", WorkerID: "test", MaxJobs: 10, TenantID: tid,
	})
	if err != nil {
		t.Fatalf("PollJobs: %v", err)
	}
	var jobID string
	for _, j := range jobs {
		if j.ProcessInstanceID == id {
			jobID = j.ID
			break
		}
	}
	if jobID == "" {
		t.Fatal("no job found")
	}

	resp := env.doRequest(t, "POST", "/jobs/"+jobID+"/fail", map[string]any{
		"retries": 2,
		"message": "transient error",
	}, tid)
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	job, err := env.s.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.State != store.StateAvailable {
		t.Errorf("expected available, got %q", job.State)
	}
	if job.Retries != 2 {
		t.Errorf("expected retries=2, got %d", job.Retries)
	}
	if job.RetryAt == nil {
		t.Error("expected RetryAt to be set")
	}
}

func TestAPI_PublishMessage(t *testing.T) {
	env := setup(t)
	tid := "message-test"
	ctx := tenant.WithID(context.Background(), tid)

	bpmn := readBPMN(t, "message-catch-throw.bpmn")
	if _, err := env.eng.DeployProcess(ctx, bpmn); err != nil {
		t.Fatalf("DeployProcess: %v", err)
	}

	id, err := env.eng.CreateInstance(ctx, "message-catch-process", map[string]any{"correlationKey": "order-123"})
	if err != nil {
		t.Fatalf("CreateInstance: %v", err)
	}
	if err := env.eng.RunInstance(ctx, id); err != nil {
		t.Fatalf("RunInstance: %v", err)
	}

	resp := env.doRequest(t, "POST", "/messages", map[string]any{
		"message_name":    "OrderPlaced",
		"correlation_key": "order-123",
		"variables":       map[string]any{"status": "received"},
	}, tid)
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	inst, err := env.s.GetInstance(ctx, id)
	if err != nil {
		t.Fatalf("GetInstance: %v", err)
	}
	if inst.State != store.StateCompleted {
		t.Errorf("expected completed, got %q", inst.State)
	}
}
