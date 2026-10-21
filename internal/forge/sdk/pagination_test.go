// SPDX-License-Identifier: GPL-3.0-or-later

package sdk

import (
	"net/http"
	"testing"
)

func TestTotalCountPresent(t *testing.T) {
	h := http.Header{}
	h.Set(totalCountHeader, "333")
	n, ok := totalCountOK(h)
	if !ok || n != 333 {
		t.Fatalf("got n=%d ok=%v, want 333/true", n, ok)
	}
}

func TestTotalCountZeroIsStillPresent(t *testing.T) {
	h := http.Header{}
	h.Set(totalCountHeader, "0")
	n, ok := totalCountOK(h)
	if !ok || n != 0 {
		t.Fatalf("got n=%d ok=%v, want 0/true", n, ok)
	}
}

func TestTotalCountAbsent(t *testing.T) {
	for name, h := range map[string]http.Header{
		"nil header":       nil,
		"empty header":     {},
		"unrelated header": {"Content-Type": []string{"application/json"}},
	} {
		t.Run(name, func(t *testing.T) {
			n, ok := totalCountOK(h)
			if ok || n != 0 {
				t.Fatalf("got n=%d ok=%v, want 0/false", n, ok)
			}
		})
	}
}

func TestTotalCountGarbage(t *testing.T) {
	for _, raw := range []string{"not-a-number", "-1", "1.5", "1e3", ""} {
		t.Run(raw, func(t *testing.T) {
			h := http.Header{}
			h.Set(totalCountHeader, raw)
			n, ok := totalCountOK(h)
			if ok || n != 0 {
				t.Fatalf("raw=%q: got n=%d ok=%v, want 0/false", raw, n, ok)
			}
		})
	}
}

func TestTotalCountPtrPresentAndAbsent(t *testing.T) {
	h := http.Header{}
	h.Set(totalCountHeader, "42")
	if p := totalCountPtr(h); p == nil || *p != 42 {
		t.Fatalf("expected pointer to 42, got %v", p)
	}

	h2 := http.Header{}
	if p := totalCountPtr(h2); p != nil {
		t.Fatalf("expected nil pointer for absent header, got %v", *p)
	}

	h3 := http.Header{}
	h3.Set(totalCountHeader, "garbage")
	if p := totalCountPtr(h3); p != nil {
		t.Fatalf("expected nil pointer for garbage header, got %v", *p)
	}
}
