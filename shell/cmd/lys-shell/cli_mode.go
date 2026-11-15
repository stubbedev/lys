package main

import (
	"fmt"
	"os"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/identity"
	"github.com/stubbedev/lys/desktop/logging"

	"github.com/stubbedev/lys/shell/internal/cli"
)

// selfManaged are the commands that run outside the shared CLI
// setup: the shell and the portal set up their own logging, and
// completions only print.
var selfManaged = map[string]bool{"shell": true, "portal": true, "completions": true}

// The daily log prefixes: the running shell and the one-shot CLI
// commands keep separate files, so neither prunes the other's.
const (
	shellLogPrefix = identity.Name
	cliLogPrefix   = "lys-shell-cli"
)

// withCLIMode wraps every other command's handler in the CLI setup:
// file-only logging ("lys-shell-cli.<day>.log") and the config
// directory. Failures are reported and ignored.
func withCLIMode(root *cli.Command) *cli.Command {
	for _, sub := range root.Subcommands {
		if !selfManaged[sub.Name] {
			wrapRuns(sub)
		}
	}
	return root
}

func wrapRuns(c *cli.Command) {
	if run := c.Run; run != nil {
		c.Run = func(m *cli.Matches) error {
			closer, err := logging.Setup(cliLogPrefix, nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to initialize tracing: %v\n", err)
			} else {
				defer func() { _ = closer.Close() }()
			}
			if err := ensureDirectories(); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to ensure directories: %v\n", err)
			}
			return run(m)
		}
	}
	for _, sub := range c.Subcommands {
		wrapRuns(sub)
	}
}

// ensureDirectories makes sure the config directory exists.
func ensureDirectories() error {
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755) //nolint:gosec // the user config dir, as create_dir_all makes it (0777 less umask)
}
