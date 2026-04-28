//go:build module_notification || all_modules
// +build module_notification all_modules

package plugin

import (
	"erg.ninja/internal/modules/notifications"
)

func init() {
	Register("notification", notifications.NewModule(notifications.Deps{}))
}
