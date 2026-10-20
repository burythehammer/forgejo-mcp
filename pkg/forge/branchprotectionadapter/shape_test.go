// SPDX-License-Identifier: GPL-3.0-or-later

package branchprotectionadapter

import (
	"encoding/json"
	"testing"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/app/model"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// TestRepoTypeMarshalsLikeSDK pins that app/model.BranchProtection renders the
// same bytes as the SDK type it mirrors, for a fully populated rule. A tool
// output built from the repo-owned type is therefore byte-identical to one
// built from the SDK.
func TestRepoTypeMarshalsLikeSDK(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	sdk := forgejo_sdk.BranchProtection{
		BranchName: "main", RuleName: "r", EnablePush: true, EnablePushWhitelist: true,
		PushWhitelistUsernames: []string{"a"}, PushWhitelistTeams: []string{"t"}, PushWhitelistDeployKeys: true,
		EnableMergeWhitelist: true, MergeWhitelistUsernames: []string{"b"}, MergeWhitelistTeams: []string{"u"},
		EnableStatusCheck: true, StatusCheckContexts: []string{"ci"}, RequiredApprovals: 2,
		EnableApprovalsWhitelist: true, ApprovalsWhitelistUsernames: []string{"c"}, ApprovalsWhitelistTeams: []string{"v"},
		BlockOnRejectedReviews: true, BlockOnOfficialReviewRequests: true, BlockOnOutdatedBranch: true,
		DismissStaleApprovals: true, RequireSignedCommits: true, ProtectedFilePatterns: "*", UnprotectedFilePatterns: "x",
		Created: ts, Updated: ts,
	}
	want, err := json.Marshal(sdk)
	if err != nil {
		t.Fatal(err)
	}
	var repo model.BranchProtection
	if err := json.Unmarshal(want, &repo); err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(repo)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("repo type differs from SDK\nsdk:  %s\nrepo: %s", want, got)
	}
}
