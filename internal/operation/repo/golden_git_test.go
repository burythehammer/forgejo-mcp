// SPDX-License-Identifier: GPL-3.0-or-later

package repo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/mcptest"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// These tests pin the wire contract of the branch, commit, contents, file and
// change_files tools: each tool's definition (schema snapshot) and its output
// and error text (golden), with the calls made on the Forgejo port. They run
// against a stub that returns fixed, fully populated data, so the bytes are
// deterministic. They were written against the hand-written handlers and must
// pass unchanged once the tools move behind their use cases.

var gitTools = []mcp.Tool{
	CreateBranchTool, DeleteBranchTool, ListBranchesTool,
	ListRepoCommitsTool,
	ListRepoContentsTool, GetRepoTreeTool,
	GetFileContentTool, CreateFileTool, UpdateFileTool, DeleteFileTool,
	ChangeFilesTool,
}

func TestGitSchemaSnapshots(t *testing.T) {
	t.Parallel()
	for _, tool := range gitTools {
		t.Run(tool.Name, func(t *testing.T) { mcptest.SchemaSnapshot(t, tool) })
	}
}

var (
	gitT0   = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	gitT1   = time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)
	gitSHA  = "0123456789abcdef0123456789abcdef01234567"
	gitSHA2 = "89abcdef0123456789abcdef0123456789abcdef"
	gitUser = &forgejo_sdk.User{ID: 1, UserName: "octo", FullName: "Octo Cat", Email: "octo@example.com", Created: gitT0, LastLogin: gitT1, IsActive: true}
	gitEnc  = "base64"
	gitB64  = "aGVsbG8K"
)

func gitBranch(name string) *forgejo_sdk.Branch {
	return &forgejo_sdk.Branch{
		Name: name,
		Commit: &forgejo_sdk.PayloadCommit{
			ID: gitSHA, Message: "init\n", URL: "https://f.example/octo/hello/commit/" + gitSHA,
			Author:       &forgejo_sdk.PayloadUser{Name: "Octo Cat", Email: "octo@example.com", UserName: "octo"},
			Committer:    &forgejo_sdk.PayloadUser{Name: "Octo Cat", Email: "octo@example.com", UserName: "octo"},
			Verification: &forgejo_sdk.PayloadCommitVerification{Verified: true, Reason: "valid", Signature: "sig", Payload: "pl"},
			Timestamp:    gitT0, Added: []string{"a.go"}, Removed: []string{}, Modified: []string{"b.go"},
		},
		Protected: true, RequiredApprovals: 1, EnableStatusCheck: true, StatusCheckContexts: []string{"ci"},
		UserCanPush: true, UserCanMerge: true, EffectiveBranchProtectionName: "main",
	}
}

func gitCommit() *forgejo_sdk.Commit {
	return &forgejo_sdk.Commit{
		CommitMeta: &forgejo_sdk.CommitMeta{URL: "https://f.example/c/" + gitSHA, SHA: gitSHA, Created: gitT0},
		HTMLURL:    "https://f.example/octo/hello/commit/" + gitSHA,
		RepoCommit: &forgejo_sdk.RepoCommit{
			URL:          "https://f.example/c/" + gitSHA,
			Author:       &forgejo_sdk.CommitUser{Identity: forgejo_sdk.Identity{Name: "Octo Cat", Email: "octo@example.com"}, Date: "2026-01-02T03:04:05Z"},
			Committer:    &forgejo_sdk.CommitUser{Identity: forgejo_sdk.Identity{Name: "Octo Cat", Email: "octo@example.com"}, Date: "2026-01-02T03:04:05Z"},
			Message:      "fix: thing\n",
			Tree:         &forgejo_sdk.CommitMeta{URL: "https://f.example/t", SHA: gitSHA2, Created: gitT0},
			Verification: &forgejo_sdk.PayloadCommitVerification{Verified: false, Reason: "no key"},
		},
		Author: gitUser, Committer: gitUser,
		Parents: []*forgejo_sdk.CommitMeta{{URL: "https://f.example/c/" + gitSHA2, SHA: gitSHA2, Created: gitT0}},
		Files:   []*forgejo_sdk.CommitAffectedFiles{{Filename: "a.go"}},
		Stats:   &forgejo_sdk.CommitStats{Total: 3, Additions: 2, Deletions: 1},
	}
}

func gitContents(path string) *forgejo_sdk.ContentsResponse {
	name := path[strings.LastIndex(path, "/")+1:]
	enc, content := gitEnc, gitB64
	url, html, git, dl := "https://f.example/api/"+path, "https://f.example/"+path, "https://f.example/git/"+path, "https://f.example/raw/"+path
	return &forgejo_sdk.ContentsResponse{
		Name: name, Path: path, SHA: gitSHA, Type: "file", Size: 6,
		Encoding: &enc, Content: &content, URL: &url, HTMLURL: &html, GitURL: &git, DownloadURL: &dl,
	}
}

func gitFileResponse(path string) *forgejo_sdk.FileResponse {
	return &forgejo_sdk.FileResponse{
		Content: gitContents(path),
		Commit: &forgejo_sdk.FileCommitResponse{
			CommitMeta: forgejo_sdk.CommitMeta{URL: "https://f.example/c/" + gitSHA, SHA: gitSHA, Created: gitT0},
			HTMLURL:    "https://f.example/c/" + gitSHA,
			Author:     &forgejo_sdk.CommitUser{Identity: forgejo_sdk.Identity{Name: "Octo Cat", Email: "octo@example.com"}, Date: "2026-01-02T03:04:05Z"},
			Committer:  &forgejo_sdk.CommitUser{Identity: forgejo_sdk.Identity{Name: "Octo Cat", Email: "octo@example.com"}, Date: "2026-01-02T03:04:05Z"},
			Parents:    []*forgejo_sdk.CommitMeta{{URL: "https://f.example/c/" + gitSHA2, SHA: gitSHA2}},
			Message:    "update\n",
			Tree:       &forgejo_sdk.CommitMeta{URL: "https://f.example/t", SHA: gitSHA2},
		},
		Verification: &forgejo_sdk.PayloadCommitVerification{Verified: true, Reason: "valid"},
	}
}

// gitStub is a forge.Forgejo whose branch, commit and file domains return
// fixed data and record the calls they receive. Any other domain panics on the
// nil embed, which would show up as a test failure.
type gitStub struct {
	forge.Forgejo
	mu       sync.Mutex
	calls    []string
	fail     error
	noDelete bool
}

func (s *gitStub) rec(name string, args ...any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, _ := json.Marshal(args)
	s.calls = append(s.calls, name+" "+string(b))
	return s.fail
}

func (s *gitStub) ListBranches(_ context.Context, o, r string, opt forgejo_sdk.ListRepoBranchesOptions) ([]*forgejo_sdk.Branch, error) {
	return []*forgejo_sdk.Branch{gitBranch("main"), gitBranch("feat")}, s.rec("ListBranches", o, r, opt)
}

func (s *gitStub) CreateBranch(_ context.Context, o, r string, opt forgejo_sdk.CreateBranchOption) (*forgejo_sdk.Branch, error) {
	return gitBranch(opt.BranchName), s.rec("CreateBranch", o, r, opt)
}

func (s *gitStub) DeleteBranch(_ context.Context, o, r, b string) (bool, error) {
	return !s.noDelete, s.rec("DeleteBranch", o, r, b)
}

func (s *gitStub) ListCommits(_ context.Context, o, r string, opt forgejo_sdk.ListCommitOptions) ([]*forgejo_sdk.Commit, error) {
	return []*forgejo_sdk.Commit{gitCommit()}, s.rec("ListCommits", o, r, opt)
}

func (s *gitStub) ListContents(_ context.Context, o, r, ref, p string) ([]*forgejo_sdk.ContentsResponse, error) {
	return []*forgejo_sdk.ContentsResponse{gitContents("docs/a.md"), gitContents("docs/sub")}, s.rec("ListContents", o, r, ref, p)
}

func (s *gitStub) GetTree(_ context.Context, o, r, ref string, opt forgejo_sdk.GetTreesOptions) (*forgejo_sdk.GitTreeResponse, error) {
	tree := &forgejo_sdk.GitTreeResponse{
		SHA: gitSHA, URL: "https://f.example/tree/" + gitSHA,
		Entries: []forgejo_sdk.GitEntry{
			{Path: "a.go", Mode: "100644", Type: "blob", Size: 12, SHA: gitSHA2, URL: "https://f.example/b/a.go"},
			{Path: "cmd", Mode: "040000", Type: "tree", SHA: gitSHA, URL: "https://f.example/t/cmd"},
		},
		Page: opt.Page, TotalCount: 2,
	}
	return tree, s.rec("GetTree", o, r, ref, opt)
}

func (s *gitStub) GetFile(_ context.Context, o, r, ref, p string) ([]byte, error) {
	return []byte("line one\nline two\nline three\n"), s.rec("GetFile", o, r, ref, p)
}

func (s *gitStub) GetContents(_ context.Context, o, r, ref, p string) (*forgejo_sdk.ContentsResponse, error) {
	return gitContents(p), s.rec("GetContents", o, r, ref, p)
}

func (s *gitStub) CreateFile(_ context.Context, o, r, p string, opt forgejo_sdk.CreateFileOptions) (*forgejo_sdk.FileResponse, error) {
	return gitFileResponse(p), s.rec("CreateFile", o, r, p, opt)
}

func (s *gitStub) UpdateFile(_ context.Context, o, r, p string, opt forgejo_sdk.UpdateFileOptions) (*forgejo_sdk.FileResponse, error) {
	return gitFileResponse(p), s.rec("UpdateFile", o, r, p, opt)
}

func (s *gitStub) DeleteFile(_ context.Context, o, r, p string, opt forgejo_sdk.DeleteFileOptions) error {
	return s.rec("DeleteFile", o, r, p, opt)
}

func (s *gitStub) ChangeFiles(_ context.Context, o, r string, opt forge.ChangeFilesOptions) (map[string]any, error) {
	out := map[string]any{"commit": map[string]any{"sha": gitSHA}, "files": len(opt.Files)}
	return out, s.rec("ChangeFiles", o, r, opt)
}

// invokeGit calls a registered tool the way the server does and renders what
// the client would see: the text, or the error, then the port calls made.
func invokeGit(t *testing.T, stub *gitStub, tool string, args map[string]any) string {
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

type gitCase struct {
	name string
	tool string
	args map[string]any
	stub func(*gitStub)
}

var (
	gitPlainFile  = map[string]any{"filePath": "docs/a.txt", "ref": "main"}
	gitFailPort   = func(s *gitStub) { s.fail = errors.New("boom") }
	gitWriteBase  = map[string]any{"filePath": "docs/a.txt", "message": "m", "branch_name": "main"}
	gitBatchFiles = []any{
		map[string]any{"operation": "create", "path": "new.txt", "content": "hi"},
		map[string]any{"operation": "update", "path": "a.txt", "content_base64": gitB64, "sha": gitSHA},
		map[string]any{"operation": "delete", "path": "old.txt", "sha": gitSHA2},
	}
)

func withArgs(base map[string]any, extra map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

var gitCases = []gitCase{
	// branch
	{name: "create_branch", tool: CreateBranchToolName, args: map[string]any{"branch": "feat", "old_branch": "main"}},
	{name: "create_branch_missing_owner", tool: CreateBranchToolName, args: map[string]any{"owner": nil, "branch": "feat"}},
	{name: "create_branch_missing_branch", tool: CreateBranchToolName, args: map[string]any{"branch": nil}},
	{name: "create_branch_missing_old_branch", tool: CreateBranchToolName, args: map[string]any{"branch": "feat", "old_branch": nil}},
	{name: "create_branch_port_error", tool: CreateBranchToolName, args: map[string]any{"branch": "feat"}, stub: gitFailPort},
	{name: "delete_branch", tool: DeleteBranchToolName, args: map[string]any{"branch": "feat"}},
	{name: "delete_branch_not_204", tool: DeleteBranchToolName, args: map[string]any{"branch": "feat"}, stub: func(s *gitStub) { s.noDelete = true }},
	{name: "delete_branch_port_error", tool: DeleteBranchToolName, args: map[string]any{"branch": "feat"}, stub: gitFailPort},
	{name: "list_branches_defaults", tool: ListBranchesToolName},
	{name: "list_branches_fractional", tool: ListBranchesToolName, args: map[string]any{"page": 2.9, "limit": "3"}},
	{name: "list_branches_port_error", tool: ListBranchesToolName, stub: gitFailPort},

	// commits
	{name: "list_commits_defaults", tool: ListRepoCommitsToolName},
	{name: "list_commits_filtered", tool: ListRepoCommitsToolName, args: map[string]any{"path": "a.go", "sha": "main", "page": float64(2), "limit": float64(5)}},
	{name: "list_commits_empty_filters", tool: ListRepoCommitsToolName, args: map[string]any{"path": "", "sha": ""}},
	{name: "list_commits_port_error", tool: ListRepoCommitsToolName, stub: gitFailPort},

	// contents and tree
	{name: "list_contents", tool: ListRepoContentsToolName, args: map[string]any{"ref": "main", "path": "docs"}},
	{name: "list_contents_root", tool: ListRepoContentsToolName, args: map[string]any{"ref": "main", "path": ""}},
	{name: "list_contents_missing_owner", tool: ListRepoContentsToolName, args: map[string]any{"owner": nil, "ref": "main"}},
	{name: "list_contents_missing_repo", tool: ListRepoContentsToolName, args: map[string]any{"repo": nil, "ref": "main"}},
	{name: "list_contents_port_error", tool: ListRepoContentsToolName, args: map[string]any{"ref": "main", "path": "docs"}, stub: gitFailPort},
	{name: "get_tree_recursive", tool: GetRepoTreeToolName, args: map[string]any{"ref": "main", "recursive": true, "page": float64(1), "limit": float64(10)}},
	{name: "get_tree_defaults", tool: GetRepoTreeToolName, args: map[string]any{"ref": "main"}},
	{name: "get_tree_recursive_as_string", tool: GetRepoTreeToolName, args: map[string]any{"ref": "main", "recursive": "true"}},
	{name: "get_tree_missing_ref", tool: GetRepoTreeToolName, args: map[string]any{"ref": nil}},
	{name: "get_tree_fractional_paging", tool: GetRepoTreeToolName, args: map[string]any{"ref": "main", "page": 2.9, "limit": 3.5}},
	{name: "get_tree_port_error", tool: GetRepoTreeToolName, args: map[string]any{"ref": "main"}, stub: gitFailPort},

	// files
	{name: "get_file_plain", tool: GetFileToolName, args: map[string]any{"ref": "main", "filePath": "docs/a.txt"}},
	{name: "get_file_line_range", tool: GetFileToolName, args: withArgs(gitPlainFile, map[string]any{"start_line": float64(2), "end_line": float64(3)})},
	{name: "get_file_only_end_line", tool: GetFileToolName, args: withArgs(gitPlainFile, map[string]any{"end_line": float64(1)})},
	{name: "get_file_range_clamped", tool: GetFileToolName, args: withArgs(gitPlainFile, map[string]any{"start_line": float64(0), "end_line": float64(99)})},
	{name: "get_file_range_inverted", tool: GetFileToolName, args: withArgs(gitPlainFile, map[string]any{"start_line": float64(3), "end_line": float64(1)})},
	{name: "get_file_with_metadata", tool: GetFileToolName, args: withArgs(gitPlainFile, map[string]any{"with_metadata": true, "start_line": float64(2)})},
	{name: "get_file_missing_file_path", tool: GetFileToolName, args: map[string]any{"filePath": nil, "ref": "main"}},
	{name: "get_file_port_error", tool: GetFileToolName, args: gitPlainFile, stub: gitFailPort},
	{name: "get_file_metadata_port_error", tool: GetFileToolName, args: withArgs(gitPlainFile, map[string]any{"with_metadata": true}), stub: gitFailPort},
	{name: "create_file_plain", tool: CreateFileToolName, args: withArgs(gitWriteBase, map[string]any{"content": "hello\n", "new_branch_name": "topic"})},
	{name: "create_file_base64", tool: CreateFileToolName, args: withArgs(gitWriteBase, map[string]any{"content_base64": gitB64})},
	{name: "create_file_empty_content", tool: CreateFileToolName, args: withArgs(gitWriteBase, map[string]any{"content": ""})},
	{name: "create_file_both_contents", tool: CreateFileToolName, args: withArgs(gitWriteBase, map[string]any{"content": "a", "content_base64": gitB64})},
	{name: "create_file_no_contents", tool: CreateFileToolName, args: gitWriteBase},
	{name: "create_file_invalid_base64", tool: CreateFileToolName, args: withArgs(gitWriteBase, map[string]any{"content_base64": "%%%"})},
	{name: "create_file_port_error", tool: CreateFileToolName, args: withArgs(gitWriteBase, map[string]any{"content": "x"}), stub: gitFailPort},
	{name: "update_file_plain", tool: UpdateFileToolName, args: withArgs(gitWriteBase, map[string]any{"content": "v2", "sha": gitSHA})},
	{name: "update_file_missing_sha", tool: UpdateFileToolName, args: withArgs(gitWriteBase, map[string]any{"content": "v2", "sha": nil})},
	{name: "update_file_port_error", tool: UpdateFileToolName, args: withArgs(gitWriteBase, map[string]any{"content": "v2", "sha": gitSHA}), stub: gitFailPort},
	{name: "delete_file", tool: DeleteFileToolName, args: withArgs(gitWriteBase, map[string]any{"sha": gitSHA})},
	{name: "delete_file_missing_sha", tool: DeleteFileToolName, args: withArgs(gitWriteBase, map[string]any{"sha": nil})},
	{name: "delete_file_missing_file_path", tool: DeleteFileToolName, args: map[string]any{"filePath": nil, "sha": gitSHA}},
	{name: "delete_file_port_error", tool: DeleteFileToolName, args: withArgs(gitWriteBase, map[string]any{"sha": gitSHA}), stub: gitFailPort},

	// change_files
	{name: "change_files_batch", tool: ChangeFilesToolName, args: map[string]any{"message": "batch", "branch_name": "main", "new_branch_name": "topic", "files": gitBatchFiles}},
	{name: "change_files_missing_message", tool: ChangeFilesToolName, args: map[string]any{"message": nil, "branch_name": "main", "files": gitBatchFiles}},
	{name: "change_files_blank_branch", tool: ChangeFilesToolName, args: map[string]any{"message": "m", "branch_name": "  ", "files": gitBatchFiles}},
	{name: "change_files_no_files", tool: ChangeFilesToolName, args: map[string]any{"message": "m", "branch_name": "main", "files": nil}},
	{name: "change_files_not_an_object", tool: ChangeFilesToolName, args: map[string]any{"message": "m", "branch_name": "main", "files": []any{"x"}}},
	{name: "change_files_duplicate_path", tool: ChangeFilesToolName, args: map[string]any{"message": "m", "branch_name": "main", "files": []any{
		map[string]any{"operation": "create", "path": "a", "content": "x"},
		map[string]any{"operation": "create", "path": "a", "content": "y"},
	}}},
	{name: "change_files_create_with_sha", tool: ChangeFilesToolName, args: map[string]any{"message": "m", "branch_name": "main", "files": []any{
		map[string]any{"operation": "create", "path": "a", "content": "x", "sha": gitSHA},
	}}},
	{name: "change_files_update_without_sha", tool: ChangeFilesToolName, args: map[string]any{"message": "m", "branch_name": "main", "files": []any{
		map[string]any{"operation": "update", "path": "a", "content": "x"},
	}}},
	{name: "change_files_delete_with_content", tool: ChangeFilesToolName, args: map[string]any{"message": "m", "branch_name": "main", "files": []any{
		map[string]any{"operation": "delete", "path": "a", "content": "x", "sha": gitSHA},
	}}},
	{name: "change_files_unknown_operation", tool: ChangeFilesToolName, args: map[string]any{"message": "m", "branch_name": "main", "files": []any{
		map[string]any{"operation": "rename", "path": "a"},
	}}},
	{name: "change_files_new_branch_wrong_type", tool: ChangeFilesToolName, args: map[string]any{"message": "m", "branch_name": "main", "new_branch_name": float64(1), "files": gitBatchFiles}},
	{name: "change_files_port_error", tool: ChangeFilesToolName, args: map[string]any{"message": "m", "branch_name": "main", "files": gitBatchFiles}, stub: gitFailPort},
}

func TestGitGolden(t *testing.T) {
	t.Parallel()
	for _, c := range gitCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			stub := &gitStub{}
			if c.stub != nil {
				c.stub(stub)
			}
			mcptest.Golden(t, "git_"+c.name, []byte(invokeGit(t, stub, c.tool, c.args)))
		})
	}
}

// MaxResponseItems reports an unknown ceiling: the repository git goldens pin the
// paging contract without a ceiling, as they always have.
func (s *gitStub) MaxResponseItems(context.Context) (int, bool) { return 0, false }
