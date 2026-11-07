// SPDX-License-Identifier: GPL-3.0-or-later

package actions

import (
	"fmt"
)

// Each tool's request pipeline decodes its arguments into a struct beside its
// run function. Strings decode as strings. Numbers are typed any, not float64:
// the tools' error texts depend on how a number was given. Each numeric
// argument is read with the numeric-argument module (internal/numarg), which
// owns the rule for whole numbers, ranges and defaults. A struct's Validate
// method holds the checks that run before the credential lookup.

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
