// SPDX-License-Identifier: GPL-3.0-or-later

package bridge

import (
	"errors"
	"testing"
)

type sdkThing struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type modelThing struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func TestConvertRoundTripsThroughJSON(t *testing.T) {
	t.Parallel()
	got, err := Convert[modelThing](&sdkThing{ID: 7, Name: "x"}, nil)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if got != (modelThing{ID: 7, Name: "x"}) {
		t.Fatalf("Convert = %+v", got)
	}
}

func TestConvertSlice(t *testing.T) {
	t.Parallel()
	got, err := Convert[[]*modelThing]([]*sdkThing{{ID: 1}, {ID: 2}}, nil)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 2 {
		t.Fatalf("Convert = %+v", got)
	}
}

func TestConvertNilPointerIsNil(t *testing.T) {
	t.Parallel()
	got, err := Convert[*modelThing]((*sdkThing)(nil), nil)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if got != nil {
		t.Fatalf("Convert = %+v, want nil", got)
	}
}

// The SDK error wins and from is not converted: a value that cannot be
// marshalled would otherwise add a second, different error.
func TestConvertPassesSDKErrorThroughUntouched(t *testing.T) {
	t.Parallel()
	sdkErr := errors.New("forge said no")
	got, err := Convert[modelThing](make(chan int), sdkErr)
	if !errors.Is(err, sdkErr) {
		t.Fatalf("err = %v, want the SDK error", err)
	}
	if got != (modelThing{}) {
		t.Fatalf("Convert = %+v, want zero value", got)
	}
}

func TestConvertReportsMarshalError(t *testing.T) {
	t.Parallel()
	if _, err := Convert[modelThing](make(chan int), nil); err == nil {
		t.Fatal("Convert of an unmarshalable value returned no error")
	}
}
