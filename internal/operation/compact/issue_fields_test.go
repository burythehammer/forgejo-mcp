// SPDX-License-Identifier: GPL-3.0-or-later

package compact

import (
	"encoding/json"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

func jsonKeys(t *testing.T, v any) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestOfIssue_MilestoneIsIDAndTitle(t *testing.T) {
	t.Parallel()
	iss := sampleIssue()
	iss.Milestone = &model.Milestone{ID: 3, Title: "v1", Description: "not echoed"}
	got := jsonKeys(t, OfIssue(iss))
	var ms map[string]any
	if err := json.Unmarshal(got["milestone"], &ms); err != nil {
		t.Fatalf("milestone: %v (%s)", err, got["milestone"])
	}
	if len(ms) != 2 || ms["id"] != float64(3) || ms["title"] != "v1" {
		t.Fatalf("milestone = %v, want exactly {id, title}", ms)
	}
}

func TestOfIssue_NoMilestoneIsNullNotAbsent(t *testing.T) {
	t.Parallel()
	got := jsonKeys(t, OfIssue(sampleIssue()))
	raw, ok := got["milestone"]
	if !ok {
		t.Fatal("milestone key must be present")
	}
	if string(raw) != "null" {
		t.Fatalf("milestone = %s, want null", raw)
	}
}

func TestOfIssue_IsPullRequestFlag(t *testing.T) {
	t.Parallel()
	iss := sampleIssue()
	if got := jsonKeys(t, OfIssue(iss))["is_pull_request"]; string(got) != "false" {
		t.Fatalf("is_pull_request for an issue = %s, want false", got)
	}
	iss.PullRequest = &model.PullRequestMeta{}
	if got := jsonKeys(t, OfIssue(iss))["is_pull_request"]; string(got) != "true" {
		t.Fatalf("is_pull_request for a pull request = %s, want true", got)
	}
}

func TestOfIssueRead_CarriesMilestoneAndFlag(t *testing.T) {
	t.Parallel()
	iss := sampleIssue()
	iss.Milestone = &model.Milestone{ID: 1, Title: "m"}
	got := jsonKeys(t, OfIssueRead(iss))
	if _, ok := got["milestone"]; !ok {
		t.Fatal("compact read lacks milestone")
	}
	if _, ok := got["is_pull_request"]; !ok {
		t.Fatal("compact read lacks is_pull_request")
	}
}

func TestOfPullRequest_KeepsItsShape(t *testing.T) {
	t.Parallel()
	pr := samplePullRequest()
	pr.Milestone = &model.Milestone{ID: 1, Title: "m"}
	got := jsonKeys(t, OfPullRequest(pr))
	for _, k := range []string{"milestone", "is_pull_request"} {
		if _, ok := got[k]; ok {
			t.Errorf("pull request result must not carry %q", k)
		}
	}
}

func TestOfDependencyPage_IsCompactIssueList(t *testing.T) {
	t.Parallel()
	iss := sampleIssue()
	p := model.DependencyPage{Issues: []*model.Issue{iss}, PageMeta: model.PageMeta{}}
	got := jsonKeys(t, OfIssueList(p.Issues, p.PageMeta))
	var issues []map[string]any
	if err := json.Unmarshal(got["issues"], &issues); err != nil || len(issues) != 1 {
		t.Fatalf("issues = %s (%v)", got["issues"], err)
	}
	if _, ok := issues[0]["body"]; ok {
		t.Fatal("dependency entries must be compact, not carry body")
	}
	if _, ok := issues[0]["is_pull_request"]; !ok {
		t.Fatal("dependency entries lack is_pull_request")
	}
}
