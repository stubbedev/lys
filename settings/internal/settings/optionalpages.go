package settings

import "github.com/stubbedev/lys/desktop/config"

var optionalModuleTools = map[config.BarModule]func() bool{
	config.ModuleTreeman: func() bool { return treemanInstalled() },
}

func unlistedOptional(cfg *config.Config, id string) bool {
	m := config.BarModule(id)
	installed, optional := optionalModuleTools[m]
	if !optional || installed() {
		return false
	}
	return cfg == nil || !layoutsHold(cfg.Bar.Layout, m)
}
