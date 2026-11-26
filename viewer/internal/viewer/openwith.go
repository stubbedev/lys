package viewer

import (
	"slices"

	"github.com/stubbedev/lys/desktop/desktopentry"
)

const AppID = "dev.stubbe.lys.viewer"

func otherApps(dirs, lists []string, mimeType string) []desktopentry.App {
	return slices.DeleteFunc(desktopentry.RecommendedFor(dirs, lists, mimeType), func(a desktopentry.App) bool {
		return a.ID == desktopentry.FileID(AppID)
	})
}
