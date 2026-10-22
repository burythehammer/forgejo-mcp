// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"testing"
	"time"
)

func TestParseRequestTimeout(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   string
		want time.Duration
		bad  bool
	}{
		{in: "", want: DefaultRequestTimeout},
		{in: "15s", want: 15 * time.Second},
		{in: "2m", want: 2 * time.Minute},
		{in: "0s", bad: true},
		{in: "-5s", bad: true},
		{in: "soon", bad: true},
		{in: "30", bad: true},
	}
	for _, tc := range cases {
		got, err := parseRequestTimeout(tc.in)
		if tc.bad {
			if err == nil {
				t.Errorf("parseRequestTimeout(%q) = %s, want an error", tc.in, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("parseRequestTimeout(%q) = %s, %v; want %s", tc.in, got, err, tc.want)
		}
	}
}

func TestRequestTimeoutFromEnvironment(t *testing.T) {
	t.Parallel()
	env := map[string]string{
		"FORGEJO_URL":             "https://forgejo.example.com",
		"FORGEJO_REQUEST_TIMEOUT": "15s",
	}
	cfg, _, err := Load(nil, func(k string) string { return env[k] }, Build{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Forgejo.RequestTimeout != 15*time.Second {
		t.Fatalf("RequestTimeout = %s, want 15s", cfg.Forgejo.RequestTimeout)
	}
}

func TestRequestTimeoutDefaultsWhenUnset(t *testing.T) {
	t.Parallel()
	env := map[string]string{"FORGEJO_URL": "https://forgejo.example.com"}
	cfg, _, err := Load(nil, func(k string) string { return env[k] }, Build{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Forgejo.RequestTimeout != DefaultRequestTimeout {
		t.Fatalf("RequestTimeout = %s, want %s", cfg.Forgejo.RequestTimeout, DefaultRequestTimeout)
	}
}
