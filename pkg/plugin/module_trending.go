//go:build module_trending || all_modules
// +build module_trending all_modules

package plugin

import (
	"erg.ninja/internal/modules/trending"
)

func init() {
	Register("trending", trending.NewModule(trending.Deps{}))
}
