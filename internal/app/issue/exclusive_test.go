// SPDX-License-Identifier: GPL-3.0-or-later

package issue

import (
	"errors"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/app/model"
)

func scoped(name string, exclusive bool) model.ScopedLabel {
	return model.ScopedLabel{CatalogLabel: model.CatalogLabel{Label: model.Label{Name: name}, Exclusive: exclusive}}
}

func TestScopeOf(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"status/ready": "status",
		"a/b/c":        "a/b",
		"plain":        "",
		"/leading":     "",
		"trailing/":    "trailing",
	}
	for name, want := range cases {
		if got := scopeOf(name); got != want {
			t.Errorf("scopeOf(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestRefuseExclusiveClash(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		labels  []model.ScopedLabel
		refused bool
	}{
		{"two exclusive same scope", []model.ScopedLabel{scoped("status/ready", true), scoped("status/done", true)}, true},
		{"same scope, not exclusive", []model.ScopedLabel{scoped("area/api", false), scoped("area/cli", false)}, false},
		{"exclusive with a non-exclusive sibling", []model.ScopedLabel{scoped("status/ready", true), scoped("status/done", false)}, false},
		{"two exclusive different scopes", []model.ScopedLabel{scoped("status/ready", true), scoped("prio/1", true)}, false},
		{"exclusive but unscoped", []model.ScopedLabel{scoped("bug", true), scoped("wip", true)}, false},
		{"exclusive with empty scope", []model.ScopedLabel{scoped("/a", true), scoped("/b", true)}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			err := refuseExclusiveClash(c.labels)
			if c.refused != (err != nil) {
				t.Fatalf("refused = %v, want %v (err %v)", err != nil, c.refused, err)
			}
			if c.refused {
				var ec *ExclusiveConflictError
				if !errors.As(err, &ec) || ec.Scope != "status" {
					t.Fatalf("err = %#v, want ExclusiveConflictError for scope status", err)
				}
			}
		})
	}
}
