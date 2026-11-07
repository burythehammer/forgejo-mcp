// SPDX-License-Identifier: GPL-3.0-or-later

package pull

import (
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation/decode"
)

func (a *listArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }

func (a *createArgs) Validate() error { return decode.RequiredRepo(a.Owner, a.Repo) }
