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

// checkOutFile refuses outFile with page or limit (outFile reads every page) and
// an outFile name that is not a bare file name. Both file-output tools call it,
// before any credential lookup.
func checkOutFile(outFile *string, hasPage, hasLimit bool) error {
	if outFile == nil {
		return nil
	}
	if hasPage {
		return errors.New("page cannot be used with outFile: outFile writes the whole listing")
	}
	if hasLimit {
		return errors.New("limit cannot be used with outFile: outFile writes the whole listing")
	}
	return outfile.CheckName(*outFile)
}

func (a *listIssuesArgs) Validate() error {
	if err := decode.Required("owner", a.Owner); err != nil {
		return err
	}
	return checkOutFile(a.OutFile, a.Page != 0, a.Limit != 0)
}

func (a *searchArgs) Validate() error {
	if err := decode.Required("owner", a.Owner); err != nil {
		return err
	}
	return checkOutFile(a.OutFile, a.Page != 0, a.Limit != 0)
}

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
