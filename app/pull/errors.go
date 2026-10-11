// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import "fmt"

// The typed error kinds of the pull use cases. They are deliberately few: the
// ones a caller can act on or test for. Failures from Forgejo are wrapped with
// the operation's name (fmt.Errorf("get pull request err: %w", err)) and keep
// their cause reachable with errors.As. The error module (#11) will
// consolidate these across domains.

// InvalidInputError is an argument the use case could not make sense of, such
// as a milestone that is not a number or review comments that are not JSON.
type InvalidInputError struct {
	// What names the argument: "invalid milestone ID".
	What string
	Err  error
}

func (e *InvalidInputError) Error() string { return fmt.Sprintf("%s: %v", e.What, e.Err) }
func (e *InvalidInputError) Unwrap() error { return e.Err }

// DiffFileNotFoundError is a file_path that is not in the pull request's diff.
type DiffFileNotFoundError struct{ Path string }

func (e *DiffFileNotFoundError) Error() string {
	return fmt.Sprintf("file_path %q not found in pull request diff", e.Path)
}

// MergeDeclinedError is a merge Forgejo answered but did not carry out.
type MergeDeclinedError struct{ StatusCode int }

func (e *MergeDeclinedError) Error() string {
	return fmt.Sprintf("merge pull request: server returned HTTP %d (expected 200)", e.StatusCode)
}
