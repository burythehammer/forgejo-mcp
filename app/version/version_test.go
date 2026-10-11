// SPDX-License-Identifier: GPL-3.0-or-later

package version

import "testing"

func TestRunReportsConfiguredVersion(t *testing.T) {
	if got := Run("1.2.3"); got.Version != "1.2.3" {
		t.Fatalf("got %q", got.Version)
	}
}

func TestRunFallsBackToDev(t *testing.T) {
	if got := Run(""); got.Version != "dev" {
		t.Fatalf("got %q", got.Version)
	}
}
