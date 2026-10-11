// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/operation/mcptest"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of every actions tool: its tool definition
// (schema snapshot) and its output and error text (golden), together with the
// calls the tool made on the Forgejo port. They run against a stub port that
// returns fixed, fully populated data, so the bytes are deterministic.

var allTools = []mcp.Tool{
	DispatchWorkflowTool, ListWorkflowRunsTool, GetWorkflowRunTool, ListActionRunJobsTool,
	GetActionJobLogsTool, CancelWorkflowRunTool, DeleteWorkflowRunTool,
	ListActionRunArtifactsTool, GetActionArtifactTool,
	ListRepoActionSecretsTool, CreateOrUpdateRepoActionSecretTool, DeleteRepoActionSecretTool,
	ListOrgActionSecretsTool, CreateOrUpdateOrgActionSecretTool, DeleteOrgActionSecretTool,
}

func TestSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range allTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

var (
	t0  = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	t1  = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	sha = "0123456789abcdef0123456789abcdef01234567"
	usr = &forgejo_sdk.User{ID: 1, UserName: "octo", FullName: "Octo Cat", Email: "octo@example.com", Created: t0, LastLogin: t1, IsActive: true}
)

func fixedRun(title string) *forgejo_sdk.ActionRun {
	return &forgejo_sdk.ActionRun{
		ID: 5, RunNumber: 2, WorkflowID: "ci.yaml", Title: title, Status: "success", Event: "push",
		CommitSHA: sha, PrettyRef: "main", HTMLURL: "https://f.example/octo/hello/actions/runs/5",
		TriggerUser: usr, Created: t0, Started: t0, Stopped: t1, Updated: t1,
	}
}

func fixedArtifact(name string) forge.ActionArtifact {
	return forge.ActionArtifact{
		ID: 8, Name: name, RunID: 5, SizeInBytes: 1024, Expired: false,
		ExpiresAt: "2026-03-01T00:00:00Z", CreatedAt: "2026-01-02T03:04:05Z", UpdatedAt: "2026-01-02T03:05:05Z",
		ArchiveDownloadURL: "https://f.example/api/v1/repos/octo/hello/actions/artifacts/8/zip",
	}
}

// stubActions is a forge.Forgejo whose actions domain returns fixed data and
// records the calls it receives. Any other domain would panic on the nil embed.
type stubActions struct {
	forge.Forgejo
	mu    sync.Mutex
	calls []string
	fail  error
	// logResp is what GetJobLog returns.
	logResp *forge.JobLog
	// total is the artifact list's X-Total-Count; nil means not reported.
	total *int
}

func (s *stubActions) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *stubActions) ActionsURL(o, r string) string {
	return "https://f.example/" + o + "/" + r + "/actions"
}

func (s *stubActions) DispatchWorkflow(_ context.Context, o, r, wf string, opt forgejo_sdk.DispatchWorkflowOption) error {
	return s.rec("DispatchWorkflow", o, r, wf, opt)
}

func (s *stubActions) ListWorkflowRuns(_ context.Context, o, r string, opt forgejo_sdk.ListActionRunsOption) (*forgejo_sdk.ListActionRunsResponse, error) {
	return &forgejo_sdk.ListActionRunsResponse{TotalCount: 2, WorkflowRuns: []*forgejo_sdk.ActionRun{fixedRun("one"), fixedRun("two")}}, s.rec("ListWorkflowRuns", o, r, opt)
}

func (s *stubActions) GetWorkflowRun(_ context.Context, o, r string, id int64) (*forgejo_sdk.ActionRun, error) {
	return fixedRun("fixed"), s.rec("GetWorkflowRun", o, r, id)
}

func (s *stubActions) CancelWorkflowRun(_ context.Context, o, r string, id int64) error {
	return s.rec("CancelWorkflowRun", o, r, id)
}

func (s *stubActions) DeleteWorkflowRun(_ context.Context, o, r string, id int64) error {
	return s.rec("DeleteWorkflowRun", o, r, id)
}

func (s *stubActions) ListRunJobs(_ context.Context, o, r string, id int64) ([]forge.ActionRunJob, error) {
	return []forge.ActionRunJob{{ID: 3, RunID: id, Attempt: 1, Handle: "h", RepoID: 9, OwnerID: 1, Name: "build", Needs: []string{"lint"}, RunsOn: []string{"ubuntu"}, TaskID: 4, Status: "success"}}, s.rec("ListRunJobs", o, r, id)
}

func (s *stubActions) GetJobLog(_ context.Context, o, r string, job, attempt int64, byteRange string, maxBytes int64) (*forge.JobLog, error) {
	return s.logResp, s.rec("GetJobLog", o, r, job, attempt, byteRange, maxBytes)
}

func (s *stubActions) ListRunArtifacts(_ context.Context, o, r string, id int64, opt forge.ArtifactListOptions) (forge.ArtifactList, error) {
	return forge.ArtifactList{Artifacts: []forge.ActionArtifact{fixedArtifact("bin"), fixedArtifact("logs")}, TotalCount: s.total}, s.rec("ListRunArtifacts", o, r, id, opt)
}

func (s *stubActions) GetArtifact(_ context.Context, o, r string, id int64) (*forge.ActionArtifact, error) {
	a := fixedArtifact("bin")
	return &a, s.rec("GetArtifact", o, r, id)
}

func (s *stubActions) ListRepoActionSecrets(_ context.Context, o, r string, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Secret, error) {
	return fixedSecrets(), s.rec("ListRepoActionSecrets", o, r, opt)
}

func (s *stubActions) SetRepoActionSecret(_ context.Context, o, r string, opt forgejo_sdk.CreateSecretOption) error {
	return s.rec("SetRepoActionSecret", o, r, opt)
}

func (s *stubActions) DeleteRepoActionSecret(_ context.Context, o, r, name string) error {
	return s.rec("DeleteRepoActionSecret", o, r, name)
}

func (s *stubActions) ListOrgActionSecrets(_ context.Context, org string, opt forgejo_sdk.ListOptions) ([]*forgejo_sdk.Secret, error) {
	return fixedSecrets(), s.rec("ListOrgActionSecrets", org, opt)
}

func (s *stubActions) SetOrgActionSecret(_ context.Context, org string, opt forgejo_sdk.CreateSecretOption) error {
	return s.rec("SetOrgActionSecret", org, opt)
}

func (s *stubActions) DeleteOrgActionSecret(_ context.Context, org, name string) error {
	return s.rec("DeleteOrgActionSecret", org, name)
}

func fixedSecrets() []*forgejo_sdk.Secret {
	return []*forgejo_sdk.Secret{{Name: "TOKEN", Data: "must-never-print", Created: t0}, {Name: "NOTIME"}}
}

// invoke calls a registered tool the way the server does and renders what the
// client would see: the text, or the error, then the port calls made.
func invoke(t *testing.T, stub *stubActions, tool string, args map[string]any) string {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterTool(s, fake.Factory{Forgejo: stub})
	st := s.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s not registered", tool)
	}
	base := map[string]any{"owner": "octo", "repo": "hello"}
	for k, v := range args {
		if v == nil {
			delete(base, k) // a nil value means "leave the argument out"
			continue
		}
		base[k] = v
	}
	res, err := st.Handler(context.Background(), mcp.CallToolRequest{Params: mcp.CallToolParams{Name: tool, Arguments: base}})
	var out string
	switch {
	case err != nil:
		out = "ERROR: " + err.Error()
	case res.IsError:
		out = "RESULT-ERROR: " + res.Content[0].(mcp.TextContent).Text
	default:
		out = res.Content[0].(mcp.TextContent).Text
	}
	return fmt.Sprintf("%s\n-- port calls --\n%s\n", out, strings.Join(stub.calls, "\n"))
}

type goldenCase struct {
	name string
	tool string
	args map[string]any
	stub func(*stubActions)
}

func tail(body string, total int) *forge.JobLog {
	return &forge.JobLog{Body: []byte(body), ContentType: "text/plain", ContentRange: fmt.Sprintf("bytes %d-%d/%d", total-len(body), total-1, total), ContentLength: int64(len(body)), StatusCode: 206}
}

var goldenCases = []goldenCase{
	{name: "dispatch_ok", tool: DispatchWorkflowToolName, args: map[string]any{"workflow": "ci.yaml", "ref": "main", "inputs": `{"a":"b"}`}},
	{name: "dispatch_no_inputs", tool: DispatchWorkflowToolName, args: map[string]any{"workflow": "ci.yaml", "ref": "main"}},
	{name: "dispatch_bad_inputs", tool: DispatchWorkflowToolName, args: map[string]any{"workflow": "ci.yaml", "ref": "main", "inputs": "{"}},
	{name: "dispatch_missing_ref", tool: DispatchWorkflowToolName, args: map[string]any{"workflow": "ci.yaml", "ref": nil}},
	{name: "dispatch_port_error", tool: DispatchWorkflowToolName, args: map[string]any{"workflow": "ci.yaml", "ref": "main"}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},

	{name: "list_runs_defaults", tool: ListWorkflowRunsToolName},
	{name: "list_runs_filtered", tool: ListWorkflowRunsToolName, args: map[string]any{"status": "success", "event": "push", "run_number": float64(2), "head_sha": sha, "page": float64(2), "limit": float64(5)}},
	{name: "list_runs_fractional", tool: ListWorkflowRunsToolName, args: map[string]any{"page": 2.9, "limit": 3.5, "run_number": "4"}},
	{name: "list_runs_missing_owner", tool: ListWorkflowRunsToolName, args: map[string]any{"owner": nil}},
	{name: "list_runs_port_error", tool: ListWorkflowRunsToolName, stub: func(s *stubActions) { s.fail = errors.New("boom") }},

	{name: "get_run", tool: GetWorkflowRunToolName, args: map[string]any{"run_id": float64(5)}},
	{name: "get_run_string_id", tool: GetWorkflowRunToolName, args: map[string]any{"run_id": "5"}},
	{name: "get_run_fractional_id", tool: GetWorkflowRunToolName, args: map[string]any{"run_id": 5.5}},
	{name: "get_run_zero_id", tool: GetWorkflowRunToolName, args: map[string]any{"run_id": float64(0)}},
	{name: "get_run_missing_id", tool: GetWorkflowRunToolName, args: map[string]any{"run_id": nil}},
	{name: "get_run_port_error", tool: GetWorkflowRunToolName, args: map[string]any{"run_id": float64(5)}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},

	{name: "jobs_defaults", tool: ListActionRunJobsToolName, args: map[string]any{"run_id": float64(5)}},
	{name: "jobs_paged", tool: ListActionRunJobsToolName, args: map[string]any{"run_id": float64(5), "page": float64(2), "limit": float64(1)}},
	{name: "jobs_bad_limit", tool: ListActionRunJobsToolName, args: map[string]any{"run_id": float64(5), "limit": float64(101)}},
	{name: "jobs_missing_run", tool: ListActionRunJobsToolName, args: map[string]any{"run_id": nil}},
	{name: "jobs_port_error", tool: ListActionRunJobsToolName, args: map[string]any{"run_id": float64(5)}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},

	{name: "log_tail_default", tool: GetActionJobLogsToolName, args: map[string]any{"job_id": float64(3)},
		stub: func(s *stubActions) { s.logResp = tail("line1\nline2\n", 100) }},
	{name: "log_offset_attempt", tool: GetActionJobLogsToolName, args: map[string]any{"job_id": float64(3), "attempt": float64(2), "offset": float64(10), "max_bytes": float64(4)},
		stub: func(s *stubActions) {
			s.logResp = &forge.JobLog{Body: []byte("abcd"), ContentType: "text/plain", ContentRange: "bytes 10-13/100", ContentLength: 4, StatusCode: 206}
		}},
	{name: "log_full_200", tool: GetActionJobLogsToolName, args: map[string]any{"job_id": "3"},
		stub: func(s *stubActions) {
			s.logResp = &forge.JobLog{Body: []byte("whole\n"), ContentType: "text/plain", ContentLength: 6, StatusCode: 200}
		}},
	{name: "log_empty", tool: GetActionJobLogsToolName, args: map[string]any{"job_id": float64(3)},
		stub: func(s *stubActions) {
			s.logResp = &forge.JobLog{ContentType: "text/plain", ContentLength: 0, StatusCode: 200}
		}},
	{name: "log_bad_content_range", tool: GetActionJobLogsToolName, args: map[string]any{"job_id": float64(3)},
		stub: func(s *stubActions) {
			s.logResp = &forge.JobLog{Body: []byte("x"), ContentRange: "garbage", StatusCode: 206}
		}},
	{name: "log_negative_offset", tool: GetActionJobLogsToolName, args: map[string]any{"job_id": float64(3), "offset": float64(-1)}},
	{name: "log_bad_attempt", tool: GetActionJobLogsToolName, args: map[string]any{"job_id": float64(3), "attempt": float64(0)}},
	{name: "log_port_error", tool: GetActionJobLogsToolName, args: map[string]any{"job_id": float64(3)}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},

	{name: "cancel_ok", tool: CancelWorkflowRunToolName, args: map[string]any{"run_id": float64(5)}},
	{name: "cancel_missing_run", tool: CancelWorkflowRunToolName, args: map[string]any{"run_id": nil}},
	{name: "cancel_port_error", tool: CancelWorkflowRunToolName, args: map[string]any{"run_id": float64(5)}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},
	{name: "delete_ok", tool: DeleteWorkflowRunToolName, args: map[string]any{"run_id": "5"}},
	{name: "delete_bad_run", tool: DeleteWorkflowRunToolName, args: map[string]any{"run_id": 1.5}},
	{name: "delete_port_error", tool: DeleteWorkflowRunToolName, args: map[string]any{"run_id": float64(5)}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},

	{name: "artifacts_defaults", tool: ListActionRunArtifactsToolName, args: map[string]any{"run_id": float64(5)}},
	{name: "artifacts_filtered_total", tool: ListActionRunArtifactsToolName, args: map[string]any{"run_id": float64(5), "page": float64(2), "limit": float64(7), "name": "bin"},
		stub: func(s *stubActions) { n := 42; s.total = &n }},
	{name: "artifacts_over_max", tool: ListActionRunArtifactsToolName, args: map[string]any{"run_id": float64(5), "limit": float64(51)}},
	{name: "artifacts_port_error", tool: ListActionRunArtifactsToolName, args: map[string]any{"run_id": float64(5)}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},
	{name: "artifact_get", tool: GetActionArtifactToolName, args: map[string]any{"artifact_id": float64(8)}},
	{name: "artifact_get_bad_id", tool: GetActionArtifactToolName, args: map[string]any{"artifact_id": "x"}},
	{name: "artifact_get_port_error", tool: GetActionArtifactToolName, args: map[string]any{"artifact_id": float64(8)}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},

	{name: "repo_secrets_list", tool: ListRepoActionSecretsToolName, args: map[string]any{"page": float64(2), "limit": float64(3)}},
	{name: "repo_secrets_list_port_error", tool: ListRepoActionSecretsToolName, stub: func(s *stubActions) { s.fail = errors.New("boom") }},
	{name: "repo_secret_set", tool: CreateOrUpdateRepoActionSecretToolName, args: map[string]any{"secret_name": "TOKEN", "data": "s3cret"}},
	{name: "repo_secret_set_missing_data", tool: CreateOrUpdateRepoActionSecretToolName, args: map[string]any{"secret_name": "TOKEN", "data": nil}},
	{name: "repo_secret_set_port_error", tool: CreateOrUpdateRepoActionSecretToolName, args: map[string]any{"secret_name": "TOKEN", "data": "x"}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},
	{name: "repo_secret_delete", tool: DeleteRepoActionSecretToolName, args: map[string]any{"secret_name": "TOKEN"}},
	{name: "repo_secret_delete_missing_name", tool: DeleteRepoActionSecretToolName, args: map[string]any{"secret_name": ""}},
	{name: "repo_secret_delete_port_error", tool: DeleteRepoActionSecretToolName, args: map[string]any{"secret_name": "TOKEN"}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},

	{name: "org_secrets_list", tool: ListOrgActionSecretsToolName, args: map[string]any{"org": "acme", "page": float64(2)}},
	{name: "org_secrets_list_missing_org", tool: ListOrgActionSecretsToolName, args: map[string]any{"org": nil}},
	{name: "org_secret_set", tool: CreateOrUpdateOrgActionSecretToolName, args: map[string]any{"org": "acme", "secret_name": "TOKEN", "data": "s3cret"}},
	{name: "org_secret_set_port_error", tool: CreateOrUpdateOrgActionSecretToolName, args: map[string]any{"org": "acme", "secret_name": "TOKEN", "data": "x"}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},
	{name: "org_secret_delete", tool: DeleteOrgActionSecretToolName, args: map[string]any{"org": "acme", "secret_name": "TOKEN"}},
	{name: "org_secret_delete_port_error", tool: DeleteOrgActionSecretToolName, args: map[string]any{"org": "acme", "secret_name": "TOKEN"}, stub: func(s *stubActions) { s.fail = errors.New("boom") }},
}

func TestGolden(t *testing.T) {
	t.Parallel()
	for _, c := range goldenCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &stubActions{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, c.name, []byte(invoke(t, stub, c.tool, c.args)))
		})
	}
}
