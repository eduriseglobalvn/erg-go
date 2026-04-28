//go:build module_bot || all_modules
// +build module_bot all_modules

package plugin

import (
	"erg.ninja/internal/modules/bot"
)

func init() {
	Register("bot", bot.NewModule(bot.Deps{}))
}
