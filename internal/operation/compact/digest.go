// SPDX-License-Identifier: GPL-3.0-or-later

package compact

import (
	"crypto/sha256"
	"encoding/hex"
)

// BodySHA256 returns the hex SHA-256 of a body. A write that sent a body reports
// the hash of the body the server stored, so an agent can confirm it was saved
// intact without the body being echoed back. Hash the stored value from the
// server's response, never the value that was sent.
func BodySHA256(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}
