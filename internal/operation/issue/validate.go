// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"errors"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/outfile"
)

// The argument structs below have no numeric argument to check, so their only
// pre-credential check is the required owner (or repo, or org) they name.
// list_issues keeps its repo check in the use case: its message names the
// owner-wide search it suggests, so it is not a plain "repo is required".

func (a *createIssueArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }

// Validate refuses outFile with page or limit (outFile reads every page) and an
// outFile name that is not a bare file name, before any credential lookup.
func (a *listIssuesArgs) Validate() error {
	if err := decode.Required("owner", a.Owner); err != nil {
		return err
	}
	if a.OutFile == nil {
		return nil
	}
	if a.Page != 0 {
		return errors.New("page cannot be used with outFile: outFile writes the whole listing")
	}
	if a.Limit != 0 {
		return errors.New("limit cannot be used with outFile: outFile writes the whole listing")
	}
	return outfile.CheckName(*a.OutFile)
}

func (a *searchArgs) Validate() error { return decode.Required("owner", a.Owner) }

func (a *milestonesArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }

func (a *repoLabelsArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }

func (a *orgLabelsArgs) Validate() error { return decode.Required("org", a.Org) }

func (a *createRepoLabelArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }

func (a *createOrgLabelArgs) Validate() error { return decode.Required("org", a.Org) }

func (a *createMilestoneArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }

func (a *editMilestoneArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }

func (a *getMilestoneArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }

func (a *addDependencyArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }

func (a *removeDependencyArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }
