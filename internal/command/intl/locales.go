package intl

import (
	"fmt"
	"strings"
)

// locales is charter intl locales: the shipped locales, comma-separated.
func locales(args []string) {
	c := app()
	fmt.Println(strings.Join(c.Locales, ","))
}
