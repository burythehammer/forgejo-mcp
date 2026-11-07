// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"context"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
	"strings"
	"testing"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

func TestRepoActionSecrets_RoundTripNeverLeaksValues(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	created := call(t, serve(h, createOrUpdateRepoActionSecret), repoArgs(map[string]any{"secret_name": "DEPLOY_TOKEN", "data": "super-secret-value"}))
	if strings.Contains(created, "super-secret-value") || !strings.Contains(created, "created_or_updated") {
		t.Fatalf("unexpected create result: %s", created)
	}

	listed := call(t, serve(h, listRepoActionSecrets), repoArgs(nil))
	if strings.Contains(listed, "super-secret-value") || !strings.Contains(listed, "DEPLOY_TOKEN") {
		t.Fatalf("list should show the name and never the value: %s", listed)
	}

	deleted := call(t, serve(h, deleteRepoActionSecret), repoArgs(map[string]any{"secret_name": "DEPLOY_TOKEN"}))
	if !strings.Contains(deleted, "deleted") {
		t.Fatalf("unexpected delete result: %s", deleted)
	}
	if listed := call(t, serve(h, listRepoActionSecrets), repoArgs(nil)); strings.Contains(listed, "DEPLOY_TOKEN") {
		t.Fatalf("deleted secret still listed: %s", listed)
	}
}

func TestListRepoActionSecretsFn_Pagination(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)
	for _, n := range []string{"AAA", "BBB", "CCC"} {
		call(t, serve(h, createOrUpdateRepoActionSecret), repoArgs(map[string]any{"secret_name": n, "data": "x"}))
	}

	got := resultOf[model.RepoSecretPage](t, serve(h, listRepoActionSecrets), repoArgs(map[string]any{"page": 2.0, "limit": 2.0}))

	if got.Count != 1 || got.Secrets[0].Name != "CCC" || got.Page != 2 || got.Limit != 2 {
		t.Fatalf("page 2: %+v", got)
	}
}

func TestCreateOrUpdateRepoActionSecretFn_MissingData(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	refusedErr(t, serve(h, createOrUpdateRepoActionSecret), repoArgs(map[string]any{"secret_name": "DEPLOY_TOKEN"}))
}

func TestDeleteRepoActionSecretFn_UnknownSecret(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	err := callErr(t, serve(h, deleteRepoActionSecret), repoArgs(map[string]any{"secret_name": "GONE"}))

	if !strings.Contains(err.Error(), "delete repo action secret") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOrgActionSecrets_RoundTripNeverLeaksValues(t *testing.T) {
	t.Parallel()
	h, f := newFixture(t)
	f.AddActionsOrg("acme")

	call(t, serve(h, createOrUpdateOrgActionSecret), map[string]any{"org": "acme", "secret_name": "NPM_TOKEN", "data": "should-not-leak"})
	listed := call(t, serve(h, listOrgActionSecrets), map[string]any{"org": "acme"})
	if strings.Contains(listed, "should-not-leak") || !strings.Contains(listed, "NPM_TOKEN") {
		t.Fatalf("list should show the name and never the value: %s", listed)
	}

	call(t, serve(h, deleteOrgActionSecret), map[string]any{"org": "acme", "secret_name": "NPM_TOKEN"})
	port, _ := h.forge.For(context.Background())
	left, err := port.ListOrgActionSecrets(context.Background(), "acme", forgejo_sdk.ListOptions{})
	if err != nil || len(left) != 0 {
		t.Fatalf("secrets left: %v %+v", err, left)
	}
}

func TestOrgActionSecrets_UnknownOrgAndMissingOrg(t *testing.T) {
	t.Parallel()
	h, _ := newFixture(t)

	callErr(t, serve(h, listOrgActionSecrets), map[string]any{"org": "ghost"})
	refusedErr(t, serve(h, createOrUpdateOrgActionSecret), map[string]any{"secret_name": "NPM_TOKEN", "data": "value"})
}
