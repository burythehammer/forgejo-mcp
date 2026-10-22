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
	pool, err := loadCAPool("--tls-ca", caFile)
	if err != nil {
		return TLS{}, err
	}
	t.CAPool = pool
	return t, nil
}

// loadCAPool reads a PEM bundle and returns the system roots with its
// certificates appended. setting names the flag in a refusal to start.
func loadCAPool(setting, caFile string) (*x509.CertPool, error) {
	pemData, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("refusing to start: %s %q cannot be read: %w", setting, caFile, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("refusing to start: cannot load the system certificate roots: %w", err)
	}
	if !pool.AppendCertsFromPEM(pemData) {
		return nil, fmt.Errorf("refusing to start: %s %q holds no valid PEM certificates", setting, caFile)
	}
	return pool, nil
}
