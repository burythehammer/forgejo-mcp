// SPDX-License-Identifier: GPL-3.0-or-later

package forge

import (
	"context"
	"time"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/pkg/log"
)

// LogAPICall logs API call information with timing.
func LogAPICall(ctx context.Context, method, endpoint string, duration time.Duration, statusCode int, err error) {
	if err != nil {
		log.ErrorCtx(ctx, "API call failed",
			log.StringField("method", method),
			log.StringField("endpoint", endpoint),
			log.DurationField("duration", duration),
			log.IntField("status_code", statusCode),
			log.ErrorField(err),
		)
	} else {
		log.DebugCtx(ctx, "API call completed",
			log.StringField("method", method),
			log.StringField("endpoint", endpoint),
			log.DurationField("duration", duration),
			log.IntField("status_code", statusCode),
		)
	}
}
