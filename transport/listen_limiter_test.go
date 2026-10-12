// SPDX-License-Identifier: GPL-3.0-or-later

package transport

import (
	"strings"
	"testing"
)

// hostRefusalMessage is the door's log line for a forged Host header. The
// isolation test counts these lines to see which listener's budget was spent.
const hostRefusalMessage = "Rejected request with unacceptable Host header"

// TestTwoListenersDoNotShareRefusalCounts pins that the refusal limiter belongs
// to the listener that serves it. A flood that spends one listener's burst must
// not silence the refusal lines of another listener in the same process.
func TestTwoListenersDoNotShareRefusalCounts(t *testing.T) {
	tr := transports()[1] // the plain Streamable HTTP transport
	app := defaultListenConfig()
	noisy := startGuarded(t, tr, app)
	quiet := startGuarded(t, tr, app)
	logs := observeLogs(t)

	for i := 0; i < refusalLogBurst*2; i++ {
		if got := get(t, noisy, tr.path, "Host: attacker.example.com"); !strings.Contains(got, "403") {
			t.Fatalf("forged Host on the noisy listener was not rejected: %q", got)
		}
	}
	before := logs.FilterMessage(hostRefusalMessage).Len()
	if before == 0 {
		t.Fatal("the noisy listener logged no refusal lines; the flood did not reach the limiter")
	}

	if got := get(t, quiet, tr.path, "Host: attacker.example.com"); !strings.Contains(got, "403") {
		t.Fatalf("forged Host on the quiet listener was not rejected: %q", got)
	}
	if after := logs.FilterMessage(hostRefusalMessage).Len(); after != before+1 {
		t.Fatalf("the quiet listener's refusal was not logged (lines %d before, %d after): "+
			"its budget is shared with the noisy listener", before, after)
	}
}

// TestRefusalLimiterIsNotPackageState pins that no package-level refusal
// limiter remains: each listener creates its own at construction, and the
// resource-server handler's debug lines go through that same instance.
func TestRefusalLimiterIsNotPackageState(t *testing.T) {
	for _, name := range []string{"listen.go", "resource_server_http.go"} {
		for _, line := range strings.Split(mustReadSource(t, name), "\n") {
			if strings.HasPrefix(line, "var ") && strings.Contains(line, "refusal") {
				t.Fatalf("%s declares package-level refusal state: %q", name, line)
			}
		}
	}
}
