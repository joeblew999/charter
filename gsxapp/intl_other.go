//go:build !js

package gsxapp

import (
	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/intl/workerd"
)

// startIntl gives a plain Go server (go run ., charter dev) the Intl a Worker has built in: workerd's, started here
// unless something set one already (a test, under intltest).
func startIntl() error {
	if intl.Runtime != nil {
		return nil
	}
	return workerd.Start()
}
