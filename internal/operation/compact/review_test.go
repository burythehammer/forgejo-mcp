// SPDX-License-Identifier: GPL-3.0-or-later

package compact

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

func sampleReview() *model.PullReview {
	return &model.PullReview{
		ID:                21,
		Reviewer:          &model.User{UserName: "matt", Email: "matt@example.test", FullName: "Matt"},
		ReviewerTeam:      &model.Team{ID: 5, Name: "core"},
		State:             "APPROVED",
		Body:              "lgtm",
		CommitID:          "0123456789abcdef0123456789abcdef01234567",
		Official:          true,
		CodeCommentsCount: 1,
		Submitted:         updated,
		HTMLURL:           "https://forge.example/o/r/pulls/7#review-21",
		HTMLPullURL:       "https://forge.example/o/r/pulls/7",
	}
}

// TestOfReviewRead_LoginNotUser: the read shape carries the reviewer as a plain
// login string and none of the verbose-only fields.
func TestOfReviewRead_LoginNotUser(t *testing.T) {
	t.Parallel()
	b, err := json.Marshal(OfReviewRead(sampleReview()))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 11 {
		t.Fatalf("compact review read has %d keys, want 11: %s", len(got), b)
	}
	if got["author"] != "matt" || got["state"] != "APPROVED" || got["comments_count"] != float64(1) {
		t.Fatalf("fields wrong: %s", b)
	}
	for _, leak := range []string{`"user"`, `"team"`, `"pull_request_url"`, "matt@example.test", "core"} {
		if strings.Contains(string(b), leak) {
			t.Fatalf("verbose-only %s leaked: %s", leak, b)
		}
	}
}

// TestOfReviewRead_NoReviewerIsEmptyLogin: a review with no reviewer renders the
// login as "", not null, so the field keeps its type.
func TestOfReviewRead_NoReviewerIsEmptyLogin(t *testing.T) {
	t.Parallel()
	r := sampleReview()
	r.Reviewer = nil
	b, err := json.Marshal(OfReviewRead(r))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if string(got["author"]) != `""` {
		t.Fatalf("author = %s, want \"\"", got["author"])
	}
}

// TestOfReviewRead_PendingSubmittedIsNull: a pending review has no submission
// time, so submitted_at renders null rather than the zero time.
func TestOfReviewRead_PendingSubmittedIsNull(t *testing.T) {
	t.Parallel()
	r := sampleReview()
	r.Submitted = time.Time{}
	b, err := json.Marshal(OfReviewRead(r))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if string(got["submitted_at"]) != "null" {
		t.Fatalf("submitted_at = %s, want null: %s", got["submitted_at"], b)
	}
}

func TestRenderReviewRead_DefaultIsCompactVerboseIsFull(t *testing.T) {
	t.Parallel()
	res, err := RenderReviewRead(sampleReview(), nil, false)
	if err != nil {
		t.Fatal(err)
	}
	got := keysOf(t, sampleTextOf(t, res))
	if got["author"] == nil || got["state"] == nil || got["user"] != nil {
		t.Fatalf("default review read keys wrong: %v", got)
	}

	res, err = RenderReviewRead(sampleReview(), nil, true)
	if err != nil {
		t.Fatal(err)
	}
	got = keysOf(t, sampleTextOf(t, res))
	if got["user"] == nil || got["team"] == nil || got["author"] != nil {
		t.Fatalf("verbose review read must be the full entity, keys: %v", got)
	}
}

func TestRenderReviewRead_ErrorIsPassedThrough(t *testing.T) {
	t.Parallel()
	res, err := RenderReviewRead(nil, errTest, false)
	if err == nil && (res == nil || !res.IsError) {
		t.Fatalf("expected an error, got res=%+v err=%v", res, err)
	}
}
