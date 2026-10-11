// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const canary = "canary-token-value-not-a-credential"

func TestSecretIsRedactedUnderEveryFormatting(t *testing.T) {
	t.Parallel()
	cfg := Config{Forgejo: Forgejo{URL: "https://f.example.org", Token: NewSecret(canary)}}
	js, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	outputs := map[string]string{
		"%v":        fmt.Sprintf("%v", cfg),
		"%+v":       fmt.Sprintf("%+v", cfg),
		"%#v":       fmt.Sprintf("%#v", cfg),
		"%s":        fmt.Sprintf("%s", cfg.Forgejo.Token),
		"%q":        fmt.Sprintf("%q", cfg.Forgejo.Token),
		"%x":        fmt.Sprintf("%x", cfg.Forgejo.Token),
		"%v ptr":    fmt.Sprintf("%v", &cfg),
		"%+v ptr":   fmt.Sprintf("%+v", &cfg),
		"json":      string(js),
		"Sprint":    fmt.Sprint(cfg.Forgejo.Token),
		"%#v token": fmt.Sprintf("%#v", cfg.Forgejo.Token),
	}
	for name, out := range outputs {
		if strings.Contains(out, canary) {
			t.Errorf("%s output leaks the token: %s", name, out)
		}
	}
	if !strings.Contains(outputs["%+v"], "[redacted]") || !strings.Contains(outputs["json"], "[redacted]") {
		t.Errorf("expected [redacted] marker, got %q and %q", outputs["%+v"], outputs["json"])
	}
}

func TestSecretIsReadOnlyThroughReveal(t *testing.T) {
	t.Parallel()
	s := NewSecret(canary)
	if s.Reveal() != canary {
		t.Fatalf("Reveal() = %q", s.Reveal())
	}
}
