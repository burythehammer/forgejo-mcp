// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"errors"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/decode"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/operation/outfile"
)

// The argument structs below have no numeric argument to check, so their only
// pre-credential check is the required owner (or repo, or org) they name.
// list_issues keeps its repo check in the use case: its message names the
// owner-wide search it suggests, so it is not a plain "repo is required".

func (a *createIssueArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }

// checkOutFileName refuses an outFile name that is not a bare file name. Both
// file-output tools call it before any credential lookup. Page and limit are
// checked on the raw request by refuseOutFilePaging, since a zero value reads
// as absent here.
func checkOutFileName(outFile *string) error {
	if outFile == nil {
		return nil
	}
	return outfile.CheckName(*outFile)
}

// checkWithBlockers refuses with_blockers without outFile: blocked_by is written
// to the file, so the argument has nothing to add to a normal page.
func checkWithBlockers(withBlockers bool, outFile *string) error {
	if withBlockers && outFile == nil {
		return errors.New("with_blockers needs outFile: blocked_by is written to the file, so it is refused without one")
	}
	return nil
}

func (a *listIssuesArgs) Validate() error {
	if err := decode.Required("owner", a.Owner); err != nil {
		return err
	}
	if err := checkWithBlockers(a.WithBlockers, a.OutFile); err != nil {
		return err
	}
	return checkOutFileName(a.OutFile)
}

func (a *searchArgs) Validate() error {
	if err := decode.Required("owner", a.Owner); err != nil {
		return err
	}
	if err := checkWithBlockers(a.WithBlockers, a.OutFile); err != nil {
		return err
	}
	return checkOutFileName(a.OutFile)
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
