// SPDX-License-Identifier: GPL-3.0-or-later

package issue_test

import (
	"context"
	"strings"
	"testing"

	apppissue "forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/issue"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// replacePort serves one issue and records the reads and writes the use case
// makes. Any other port method panics on the nil embed, which these tests do
// not reach.
type replacePort struct {
	apppissue.Port
	issue *model.Issue
	gets  int
	edits []apppissue.IssueEdit
}

func (p *replacePort) GetIssue(_ context.Context, _, _ string, _ int64) (*model.Issue, error) {
	p.gets++
	return p.issue, nil
}

func (p *replacePort) EditIssue(_ context.Context, _, _ string, _ int64, in apppissue.IssueEdit) (*model.Issue, error) {
	p.edits = append(p.edits, in)
	return &model.Issue{Body: *in.Body}, nil
}

var replaceTarget = apppissue.Target{Owner: "o", Repo: "r", Index: 3}

func TestReplaceBodyLine_OneMatchIsOneReadAndOneWrite(t *testing.T) {
	t.Parallel()
	p := &replacePort{issue: &model.Issue{Body: "alpha beta gamma"}}
	got, err := apppissue.New(p).ReplaceBodyLine(context.Background(), apppissue.ReplaceBodyLineInput{
		Target: replaceTarget, Old: "beta", New: "BETA",
	})
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if p.gets != 1 {
		t.Errorf("GetIssue calls = %d, want 1", p.gets)
	}
	if len(p.edits) != 1 {
		t.Fatalf("EditIssue calls = %d, want 1", len(p.edits))
	}
	if body := *p.edits[0].Body; body != "alpha BETA gamma" {
		t.Errorf("edited body = %q, want %q", body, "alpha BETA gamma")
	}
	if got == nil || got.Body != "alpha BETA gamma" {
		t.Errorf("returned issue = %+v, want the edited body", got)
	}
}

func TestReplaceBodyLine_RefusesWithoutWriting(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		issue   *model.Issue
		in      apppissue.ReplaceBodyLineInput
		wantErr string
	}{
		{
			name:    "zero matches",
			issue:   &model.Issue{Body: "alpha beta"},
			in:      apppissue.ReplaceBodyLineInput{Target: replaceTarget, Old: "delta", New: "x"},
			wantErr: "not found",
		},
		{
			name:    "many matches",
			issue:   &model.Issue{Body: "x and x"},
			in:      apppissue.ReplaceBodyLineInput{Target: replaceTarget, Old: "x", New: "y"},
			wantErr: "occurs 2 times",
		},
		{
			name:    "empty old",
			issue:   &model.Issue{Body: "alpha"},
			in:      apppissue.ReplaceBodyLineInput{Target: replaceTarget, Old: "", New: "y"},
			wantErr: "must not be empty",
		},
		{
			name:    "pull request",
			issue:   &model.Issue{Body: "alpha", PullRequest: &model.PullRequestMeta{}},
			in:      apppissue.ReplaceBodyLineInput{Target: replaceTarget, Old: "alpha", New: "beta"},
			wantErr: "is a pull request",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			p := &replacePort{issue: c.issue}
			_, err := apppissue.New(p).ReplaceBodyLine(context.Background(), c.in)
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("error %q, want it to contain %q", err, c.wantErr)
			}
			if len(p.edits) != 0 {
				t.Errorf("EditIssue called on a refused replace: %+v", p.edits)
			}
		})
	}
}
