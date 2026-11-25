//go:build js

package site

// startIntl does nothing on a Worker: Intl is the runtime's own.
func startIntl() error { return nil }
