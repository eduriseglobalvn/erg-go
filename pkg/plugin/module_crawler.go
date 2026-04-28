//go:build module_crawler || all_modules
// +build module_crawler all_modules

package plugin

import (
	"erg.ninja/internal/modules/crawler"
)

func init() {
	Register("crawler", crawler.NewModule(crawler.Deps{}))
}
