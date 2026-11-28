package locale

import (
	"testing"

	"github.com/stubbedev/lys/desktop/i18n/i18ntest"
)

func TestTheCataloguesAreComplete(t *testing.T) {
	i18ntest.CheckAssets(t, Assets(), "viewer-trashed")
}
