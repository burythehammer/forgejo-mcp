// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import "fmt"

// required returns the error for a string argument that is missing or empty.
func required(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

// requiredRepo checks owner, then repo, the order the tools always used.
func requiredRepo(owner, repo string) error {
	if err := required("owner", owner); err != nil {
		return err
	}
	return required("repo", repo)
}

// The argument structs below have no numeric argument to check, so their only
// pre-credential check is the required owner (or repo, or org) they name.
// list_issues keeps its repo check in the use case: its message names the
// owner-wide search it suggests, so it is not a plain "repo is required".

func (a *createIssueArgs) Validate() error { return requiredRepo(a.Owner, a.Repo) }

func (a *listIssuesArgs) Validate() error { return required("owner", a.Owner) }

func (a *searchArgs) Validate() error { return required("owner", a.Owner) }

func (a *milestonesArgs) Validate() error { return requiredRepo(a.Owner, a.Repo) }

func (a *repoLabelsArgs) Validate() error { return requiredRepo(a.Owner, a.Repo) }

func (a *orgLabelsArgs) Validate() error { return required("org", a.Org) }

func (a *createRepoLabelArgs) Validate() error { return requiredRepo(a.Owner, a.Repo) }

func (a *createOrgLabelArgs) Validate() error { return required("org", a.Org) }

func (a *createMilestoneArgs) Validate() error { return requiredRepo(a.Owner, a.Repo) }

func (a *editMilestoneArgs) Validate() error { return requiredRepo(a.Owner, a.Repo) }

func (a *getMilestoneArgs) Validate() error { return requiredRepo(a.Owner, a.Repo) }

func (a *addDependencyArgs) Validate() error { return requiredRepo(a.Owner, a.Repo) }

func (a *removeDependencyArgs) Validate() error { return requiredRepo(a.Owner, a.Repo) }
