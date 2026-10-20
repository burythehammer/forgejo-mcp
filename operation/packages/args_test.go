// SPDX-License-Identifier: GPL-3.0-or-later

package packages

import (
	"testing"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/operation/mcptest"
)

// TestArgsMatchSchema keeps each tool's argument struct in agreement with its
// input schema: same names, types and required set.
func TestArgsMatchSchema(t *testing.T) {
	t.Parallel()
	mcptest.ArgsMatchSchema(t, ListPackagesTool, listPackagesArgs{})
	mcptest.ArgsMatchSchema(t, GetPackageTool, packageArgs{})
	mcptest.ArgsMatchSchema(t, DeletePackageTool, packageArgs{})
	mcptest.ArgsMatchSchema(t, ListPackageFilesTool, listPackageFilesArgs{})
}
