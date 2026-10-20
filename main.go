// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"runtime/debug"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/cmd"
)

var Version = "dev"

func main() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}
	cmd.Execute(Version)
}
