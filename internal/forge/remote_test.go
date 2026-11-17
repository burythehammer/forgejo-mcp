// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"
	"testing"
)

func TestMarkRemote(t *testing.T) {
	t.Parallel()
	if Remote(context.Background()) {
		t.Fatal("an unmarked context reads as remote")
	}
	if !Remote(MarkRemote(context.Background())) {
		t.Fatal("a marked context does not read as remote")
	}
}
