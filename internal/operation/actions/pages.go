// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/list"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/app/model"
)

// runPage is one page of workflow runs in the list tool's wire form.
func runPage(b list.Bounded[model.WorkflowRunRow]) model.WorkflowRunPage {
	m := model.PageMetaOf(b)
	return model.WorkflowRunPage{WorkflowRuns: b.Items, PageWindow: m.PageWindow, PageTotal: m.PageTotal, PageNext: m.PageNext}
}

// jobPage is one page of a run's jobs in the list tool's wire form. next_page
// names the page after this one, and is set only when more rows follow.
func jobPage(runID int64, b list.Bounded[model.ActionRunJob]) model.JobPage {
	m := model.PageMetaOf(b)
	out := model.JobPage{RunID: runID, Jobs: b.Items, PageWindow: m.PageWindow, PageTotal: m.PageTotal, PageNext: m.PageNext}
	if out.HasNext {
		out.NextPage = b.Page + 1
	}
	return out
}

// artifactPage is one page of a run's artifacts in the list tool's wire form.
func artifactPage(runID int64, b list.Bounded[model.ActionArtifact]) model.ArtifactPage {
	m := model.PageMetaOf(b)
	return model.ArtifactPage{RunID: runID, Artifacts: b.Items, PageWindow: m.PageWindow, PageTotal: m.PageTotal, PageNext: m.PageNext}
}

// repoSecretPage is one page of a repository's secret names in its wire form.
func repoSecretPage(owner, repo string, b list.Bounded[model.SecretSummary]) model.RepoSecretPage {
	return model.RepoSecretPage{Owner: owner, Repo: repo, Secrets: b.Items, PageMeta: model.PageMetaOf(b)}
}

// orgSecretPage is one page of an organization's secret names in its wire form.
func orgSecretPage(org string, b list.Bounded[model.SecretSummary]) model.OrgSecretPage {
	return model.OrgSecretPage{Org: org, Secrets: b.Items, PageMeta: model.PageMetaOf(b)}
}
