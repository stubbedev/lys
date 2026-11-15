package main

import (
	"github.com/stubbedev/lys/shell/internal/cli"
	"github.com/stubbedev/lys/shell/internal/lockcli"
)

func lockCommand() *cli.Command { return lockcli.Command("lock") }
