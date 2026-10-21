// SPDX-License-Identifier: GPL-3.0-or-later

package decode

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

type argHolder struct {
	Text  Arg `json:"text"`
	Count Arg `json:"count"`
}

func lax(t *testing.T, args map[string]any) argHolder {
	t.Helper()
	var a argHolder
	req := mcp.CallToolRequest{Params: mcp.CallToolParams{Arguments: args}}
	if err := Lax(req, &a); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestArgPresence(t *testing.T) {
	t.Parallel()
	if a := lax(t, map[string]any{}); a.Text.Present() {
		t.Error("absent argument reads as present")
	}
	if a := lax(t, map[string]any{"text": nil}); !a.Text.Present() {
		t.Error("null argument must read as present")
	}
	if a := lax(t, map[string]any{"text": "x"}); !a.Text.Present() {
		t.Error("given argument reads as absent")
	}
}

func TestArgString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    map[string]any
		want    string
		ok      bool
		wantErr string
	}{
		{name: "absent", args: map[string]any{}},
		{name: "value", args: map[string]any{"text": "hi"}, want: "hi", ok: true},
		{name: "explicit empty", args: map[string]any{"text": ""}, ok: true},
		{name: "null", args: map[string]any{"text": nil}, ok: true, wantErr: "text must be a string"},
		{name: "number", args: map[string]any{"text": float64(1)}, ok: true, wantErr: "text must be a string"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := lax(t, tt.args)
			got, ok, err := a.Text.String("text")
			if got != tt.want || ok != tt.ok {
				t.Fatalf("got (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
			if tt.wantErr == "" && err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr) {
				t.Fatalf("error %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestArgInt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		args    map[string]any
		want    int
		wantErr string
	}{
		{name: "absent takes default", args: map[string]any{}, want: 7},
		{name: "null takes default", args: map[string]any{"count": nil}, want: 7},
		{name: "empty string takes default", args: map[string]any{"count": ""}, want: 7},
		{name: "number", args: map[string]any{"count": float64(3)}, want: 3},
		{name: "numeric string", args: map[string]any{"count": "4"}, want: 4},
		{name: "fraction refused", args: map[string]any{"count": 2.5}, wantErr: "count must be an integer between 1 and 50"},
		{name: "below minimum", args: map[string]any{"count": float64(0)}, wantErr: "count must be an integer between 1 and 50"},
		{name: "above maximum", args: map[string]any{"count": float64(51)}, wantErr: "count must be an integer between 1 and 50"},
		{name: "not a number", args: map[string]any{"count": "many"}, wantErr: "count must be an integer between 1 and 50"},
		{name: "bool refused", args: map[string]any{"count": true}, wantErr: "count must be an integer between 1 and 50"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := lax(t, tt.args)
			got, err := a.Count.Int("count", 7, 1, 50)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("error %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got (%d, %v), want (%d, nil)", got, err, tt.want)
			}
		})
	}
}

func TestMapDecodesAPlainArgumentMap(t *testing.T) {
	t.Parallel()
	var a argHolder
	if err := Map(map[string]any{"text": "x"}, &a); err != nil {
		t.Fatal(err)
	}
	if got, ok, err := a.Text.String("text"); got != "x" || !ok || err != nil {
		t.Fatalf("got (%q, %v, %v)", got, ok, err)
	}
	if err := Map(map[string]any{}, a); err == nil {
		t.Fatal("a non-pointer destination must be refused")
	}
}
