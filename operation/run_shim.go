// SPDX-License-Identifier: GPL-3.0-or-later

package operation

import (
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/config"
	"git.b4mad.industries/agentic-forges/forgejo-mcp/v3/pkg/flag"
)

// Run serves the settings held in pkg/flag on transport.
//
// TEMPORARY: it rebuilds a Config from the globals cmd still fills, so the
// listeners can read a Config now. #42 gives Run the loaded Config and deletes
// this file together with pkg/flag.
func Run(transport, version string) error {
	return RunConfig(configFromFlagPkg(transport), version)
}

func setLegacyVersion(version string) { flag.Version = version }

func configFromFlagPkg(transport string) config.Config {
	cfg := config.Config{
		Mode:      config.ModeServe,
		Transport: transport,
		Forgejo: config.Forgejo{
			URL:       flag.URL,
			Token:     config.NewSecret(flag.Token),
			UserAgent: flag.UserAgent,
		},
		Listen: config.Listen{
			SSEPort:                    flag.SSEPort,
			HTTPPort:                   flag.HTTPPort,
			Host:                       flag.Host,
			AllowedHosts:               flag.AllowedHosts,
			AllowedOrigins:             flag.AllowedOrigins,
			AllowOperatorTokenFallback: flag.AllowOperatorTokenFallback,
		},
		Auth: config.Auth{
			Mode:                        flag.AuthMode,
			AuthorizationServer:         flag.AuthorizationServer,
			Resource:                    flag.Resource,
			ResourceAudience:            flag.ResourceAudience,
			ScopesSupported:             flag.ScopesSupported,
			ForgejoAudienceClaim:        flag.ForgejoAudienceClaim,
			ForgejoJWTIssuer:            flag.ForgejoJWTIssuer,
			ForgejoJWTSigningKeyFile:    flag.ForgejoJWTSigningKeyFile,
			ForgejoJWTPublishedKeyFiles: flag.ForgejoJWTPublishedKeyFiles,
		},
		Debug: flag.Debug,
	}
	return cfg.WithSuppliedResourceServerSettings(flag.ResourceServerSettings)
}
