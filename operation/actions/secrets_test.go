// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/app/model"
	"strings"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func TestRepoActionSecrets_RoundTripNeverLeaksValues(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	created := call(t, h.CreateOrUpdateRepoActionSecretFn, repoArgs(map[string]any{"secret_name": "DEPLOY_TOKEN", "data": "super-secret-value"}))
	if strings.Contains(created, "super-secret-value") || !strings.Contains(created, "created_or_updated") {
		t.Fatalf("unexpected create result: %s", created)
	}

	listed := call(t, h.ListRepoActionSecretsFn, repoArgs(nil))
	if strings.Contains(listed, "super-secret-value") || !strings.Contains(listed, "DEPLOY_TOKEN") {
		t.Fatalf("list should show the name and never the value: %s", listed)
	}

	deleted := call(t, h.DeleteRepoActionSecretFn, repoArgs(map[string]any{"secret_name": "DEPLOY_TOKEN"}))
	if !strings.Contains(deleted, "deleted") {
		t.Fatalf("unexpected delete result: %s", deleted)
	}
	if listed := call(t, h.ListRepoActionSecretsFn, repoArgs(nil)); strings.Contains(listed, "DEPLOY_TOKEN") {
		t.Fatalf("deleted secret still listed: %s", listed)
	}
}

func TestListRepoActionSecretsFn_Pagination(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	for _, n := range []string{"AAA", "BBB", "CCC"} {
		call(t, h.CreateOrUpdateRepoActionSecretFn, repoArgs(map[string]any{"secret_name": n, "data": "x"}))
	}

	got := decode[model.RepoSecretPage](t, h.ListRepoActionSecretsFn, repoArgs(map[string]any{"page": 2.0, "limit": 2.0}))

	if got.Count != 1 || got.Secrets[0].Name != "CCC" || got.Page != 2 || got.Limit != 2 {
		t.Fatalf("page 2: %+v", got)
	}
}

func TestCreateOrUpdateRepoActionSecretFn_MissingData(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	callErr(t, h.CreateOrUpdateRepoActionSecretFn, repoArgs(map[string]any{"secret_name": "DEPLOY_TOKEN"}))
}

func TestDeleteRepoActionSecretFn_UnknownSecret(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, h.DeleteRepoActionSecretFn, repoArgs(map[string]any{"secret_name": "GONE"}))

	if !strings.Contains(err.Error(), "delete repo action secret") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOrgActionSecrets_RoundTripNeverLeaksValues(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddActionsOrg("acme")

	call(t, h.CreateOrUpdateOrgActionSecretFn, map[string]any{"org": "acme", "secret_name": "NPM_TOKEN", "data": "should-not-leak"})
	listed := call(t, h.ListOrgActionSecretsFn, map[string]any{"org": "acme"})
	if strings.Contains(listed, "should-not-leak") || !strings.Contains(listed, "NPM_TOKEN") {
		t.Fatalf("list should show the name and never the value: %s", listed)
	}

	call(t, h.DeleteOrgActionSecretFn, map[string]any{"org": "acme", "secret_name": "NPM_TOKEN"})
	port, _ := h.forge.For(context.Background())
	left, err := port.ListOrgActionSecrets(context.Background(), "acme", forgejo_sdk.ListOptions{})
	if err != nil || len(left) != 0 {
		t.Fatalf("secrets left: %v %+v", err, left)
	}
}

func TestOrgActionSecrets_UnknownOrgAndMissingOrg(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	callErr(t, h.ListOrgActionSecretsFn, map[string]any{"org": "ghost"})
	callErr(t, h.CreateOrUpdateOrgActionSecretFn, map[string]any{"secret_name": "NPM_TOKEN", "data": "value"})
}
