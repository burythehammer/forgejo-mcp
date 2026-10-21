// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"context"
	"errors"
	stdflag "flag"
	"fmt"
	"os"

	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/config"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/log"
	"forgejo.burythehammer.com/burythehammer/forgejo-mcp/v3/internal/operation"
)

// Execute is the composition root: the one place that loads the configuration,
// logs its warnings, configures logging, initialises the Forgejo client and
// starts the server or the CLI. A configuration that fails to load or validate
// stops here, before anything is bound or contacted.
func Execute(version string) {
	args := os.Args[1:]
	if config.IsVersionRequest(args) {
		fmt.Printf("forgejo-mcp %s\n", version)
		return
	}

	cfg, warns, err := config.Load(args, os.Getenv, config.Build{Version: version})
	if err != nil {
		var usage *config.UsageError
		if errors.As(err, &usage) {
			fmt.Fprint(os.Stderr, usage.Usage)
			if errors.Is(err, stdflag.ErrHelp) {
				os.Exit(0)
			}
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		log.Fatal("Invalid configuration", log.ErrorField(err))
	}

	log.Configure(cfg.Debug)
	for _, w := range warns {
		log.Warn("Deprecated environment variable used",
			log.StringField("deprecated_var", w.Deprecated),
			log.StringField("preferred_var", w.Preferred),
			log.StringField("migration_help", "Please update your configuration to use "+w.Preferred),
		)
	}

	// Sync flushes buffered logs at exit; its error (e.g. syncing stderr) is
	// not actionable here.
	defer func() { _ = log.Default().Sync() }()

	if cfg.Mode == config.ModeCLI {
		RunCLI(cfg, version)
		return
	}

	log.Infof("Starting Forgejo MCP Server %s", version)
	log.Info("Server configuration loaded",
		log.SanitizedURLField("url", cfg.Forgejo.URL),
		log.StringField("transport", cfg.Transport),
		log.IntField("sse-port", cfg.Listen.SSEPort),
		log.BoolField("debug", cfg.Debug),
		log.BoolField("token_configured", cfg.Forgejo.Token.Reveal() != ""),
		log.StringField("user_agent", cfg.Forgejo.UserAgent),
	)

	if err := operation.Run(cfg, version); err != nil {
		if errors.Is(err, context.Canceled) {
			log.Info("Server shutdown due to context cancellation")
			return
		}
		log.Fatalf("Run Forgejo MCP Server Error: %v", err)
	}
}
