// SPDX-License-Identifier: GPL-3.0-or-later

// Package bridge holds the one conversion every pkg/forge/*adapter shares: an
// SDK value becomes its repo-owned twin in app/model through its JSON form,
// which is exactly what a tool renders. Keeping it here means a field the
// repo-owned type shares by JSON name cannot drift between domains.
package bridge

import "encoding/json"

// Convert maps an SDK value onto its repo-owned twin through JSON. The error of
// the SDK call is passed through first: when err is non-nil the zero T and that
// error are returned and from is not touched, so a bridge can end with
// `return bridge.Convert[T](x, err)` after the SDK call.
func Convert[T any](from any, err error) (T, error) {
	var to T
	if err != nil {
		return to, err
	}
	b, err := json.Marshal(from)
	if err != nil {
		return to, err
	}
	err = json.Unmarshal(b, &to)
	return to, err
}
