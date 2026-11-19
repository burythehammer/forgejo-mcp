// SPDX-License-Identifier: GPL-3.0-or-later

package owneradapter

import (
	"context"
	"errors"
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v4/internal/forge/fake"

	forgejo_sdk "codeberg.org/mvdkleijn/forgejo-sdk/forgejo/v3"
)

// userFailure is a Users port whose lookup fails with err and nothing else.
type userFailure struct {
	*fake.Forgejo
	err error
}

func (u userFailure) GetUserInfo(context.Context, string) (*forgejo_sdk.User, error) {
	return nil, u.err
}

// Only a Forgejo 404 is a user miss. A failure whose text merely mentions
// "not found" is not, and must reach the caller so the owner read does not fall
// back to the org as if the login were unknown.
func TestGetUser_OnlyAForgejo404IsAMiss(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		err      error
		wantMiss bool
	}{
		{"typed 404 is a miss", &forge.StatusError{StatusCode: 404, Err: errors.New("user does not exist")}, true},
		{"500 whose text says not found is not a miss", &forge.StatusError{StatusCode: 500, Err: errors.New("repository not found in index")}, false},
		{"plain error whose text says not found is not a miss", errors.New("user not found"), false},
		{"plain error with 404 in its text is not a miss", errors.New("upstream answered 404 to a proxy"), false},
		{"403 is not a miss", &forge.StatusError{StatusCode: 403, Err: errors.New("forbidden")}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a := New(userFailure{Forgejo: fake.New(), err: c.err}, fake.New())
			_, err := a.GetUser(context.Background(), "octo")
			if got := errors.Is(err, forge.ErrNotFound); got != c.wantMiss {
				t.Fatalf("errors.Is(err, forge.ErrNotFound) = %v, want %v (err = %v)", got, c.wantMiss, err)
			}
			if err == nil {
				t.Fatal("want an error")
			}
		})
	}
}
