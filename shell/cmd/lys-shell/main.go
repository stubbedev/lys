// Command lys-shell is the lys-shell CLI: its command tree (help,
// flags, errors, exit codes, shell completions) is declared once in
// internal/cli form. Each top-level subcommand lives in its own file as
// a <name>Command constructor.
package main

import (
	"os"
	"path/filepath"

	"github.com/stubbedev/lys/lib/buildinfo"

	"github.com/stubbedev/lys/shell/internal/cli"
)

// rootCommand is the CLI: the subcommands in the order help and
// completions list them.
func rootCommand() *cli.Command {
	return &cli.Command{
		Name:    "lys-shell",
		About:   "Wayland services CLI",
		Version: buildinfo.Version(),
		Subcommands: []*cli.Command{
			audioCommand(),
			brightnessCommand(),
			configCommand(),
			iconsCommand(),
			mediaCommand(),
			notifyCommand(),
			panelCommand(),
			powerCommand(),
			systrayCommand(),
			wallpaperCommand(),
			idleCommand(),
			launcherCommand(),
			lockCommand(),
			logoutCommand(),
			vpnCommand(),
			recorderCommand(),
			screenshotCommand(),
			colorpickerCommand(),
			widgetCommand(),
			toastCommand(),
			portalCommand(),
			shellCommand(),
			completionsCommand(),
		},
	}
}

func main() {
	os.Exit(run(os.Args, cli.StdOutput()))
}

// run is main without the process exit. Invoked as `rofi` (a symlink
// or hardlink), the whole argument vector is rofi-style launcher flags.
func run(argv []string, out cli.Output) int {
	args := argv[1:]
	if len(argv) > 0 && filepath.Base(argv[0]) == "rofi" {
		args = append([]string{"launcher"}, args...)
	}
	return cli.Run(withCLIMode(rootCommand()), args, out)
}
