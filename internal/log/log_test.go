// SPDX-License-Identifier: GPL-3.0-or-later

package log

import (
	"testing"

	"go.uber.org/zap/zapcore"
)

func TestConfigureSetsDebugLevel(t *testing.T) {
	Configure(true)
	if !Default().Core().Enabled(zapcore.DebugLevel) {
		t.Fatal("debug level not enabled after Configure(true)")
	}
	Configure(false)
	if Default().Core().Enabled(zapcore.DebugLevel) {
		t.Fatal("debug level still enabled after Configure(false)")
	}
	if !Default().Core().Enabled(zapcore.InfoLevel) {
		t.Fatal("info level not enabled after Configure(false)")
	}
}
