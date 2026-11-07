// SPDX-License-Identifier: GPL-3.0-or-later

package pull

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

func (a *listArgs) Validate() error { return requiredRepo(a.Owner, a.Repo) }

func (a *createArgs) Validate() error { return requiredRepo(a.Owner, a.Repo) }
