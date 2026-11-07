// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import "fmt"

// Required returns the error for a string argument that is missing or empty.
// It is the one check every argument struct's Validate method uses for a
// required string, so the wording is the same in every domain.
func Required(name, value string) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

// RequiredRepo checks owner, then repo, the order the tools always used.
func RequiredRepo(owner, repo string) error {
	if err := Required("owner", owner); err != nil {
		return err
	}
	return Required("repo", repo)
}
