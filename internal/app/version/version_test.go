// SPDX-License-Identifier: GPL-3.0-or-later

package version

import (
	"context"
	"errors"
	"testing"
)

func TestRunReportsConfiguredVersion(t *testing.T) {
	t.Parallel()
	if got := Run("1.2.3"); got.Version != "1.2.3" {
		t.Fatalf("got %q", got.Version)
	}
}

func TestRunFallsBackToDev(t *testing.T) {
	t.Parallel()
	if got := Run(""); got.Version != "dev" {
		t.Fatalf("got %q", got.Version)
	}
}

// stubProber stands in for the Forgejo port's ProbeServerVersion.
type stubProber struct {
	version string
	err     error
}

func (s stubProber) ProbeServerVersion(context.Context) (string, error) {
	return s.version, s.err
}

func TestProbeForgejoReturnsTheVersionVerbatim(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"16.0.0", "17.0.0-rc1", "16.0.0+gitea-1.22.0"} {
		got, err := ProbeForgejo(context.Background(), stubProber{version: v})
		if err != nil {
			t.Fatalf("ProbeForgejo(%q): %v", v, err)
		}
		if got.Version != v {
			t.Fatalf("got %q, want %q unparsed", got.Version, v)
		}
	}
}

func TestProbeForgejoReturnsTheProbeError(t *testing.T) {
	t.Parallel()
	want := errors.New("probe failed")
	if _, err := ProbeForgejo(context.Background(), stubProber{err: want}); !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}
