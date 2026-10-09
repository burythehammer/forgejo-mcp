// SPDX-License-Identifier: GPL-3.0-or-later

package config

import "fmt"

const redacted = "[redacted]"

// Secret holds a credential. Every formatting verb and JSON encoding prints
// [redacted]; the value is read only through Reveal.
type Secret struct {
	value string
}

// NewSecret wraps value.
func NewSecret(value string) Secret { return Secret{value: value} }

// Reveal returns the credential. Call it only where the value is used.
func (s Secret) Reveal() string { return s.value }

// IsSet reports whether a credential was supplied.
func (s Secret) IsSet() bool { return s.value != "" }

// String implements fmt.Stringer.
func (Secret) String() string { return redacted }

// GoString implements fmt.GoStringer, for %#v.
func (Secret) GoString() string { return redacted }

// Format implements fmt.Formatter so no verb can reach the value.
func (Secret) Format(f fmt.State, _ rune) { _, _ = f.Write([]byte(redacted)) }

// MarshalJSON implements json.Marshaler.
func (Secret) MarshalJSON() ([]byte, error) { return []byte(`"` + redacted + `"`), nil }

// MarshalText implements encoding.TextMarshaler.
func (Secret) MarshalText() ([]byte, error) { return []byte(redacted), nil }
