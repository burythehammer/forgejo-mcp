// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"crypto/x509"
	"fmt"
	"os"
)

// loadTLS builds the TLS settings for a Forgejo instance. A bundle is read now,
// so an unreadable file or one with no certificates stops startup rather than
// failing on the first request. The bundle is appended to the system roots.
func loadTLS(caFile string, skipVerify bool) (TLS, error) {
	t := TLS{SkipVerify: skipVerify}
	if caFile == "" {
		return t, nil
	}
	pemData, err := os.ReadFile(caFile)
	if err != nil {
		return TLS{}, fmt.Errorf("refusing to start: --tls-ca %q cannot be read: %w", caFile, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		return TLS{}, fmt.Errorf("refusing to start: cannot load the system certificate roots: %w", err)
	}
	if !pool.AppendCertsFromPEM(pemData) {
		return TLS{}, fmt.Errorf("refusing to start: --tls-ca %q holds no valid PEM certificates", caFile)
	}
	t.CAPool = pool
	return t, nil
}
