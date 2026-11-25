package intl

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/joeblew999/charter/internal/intlkit/catalog"
	"github.com/joeblew999/charter/site/intl"
	"github.com/joeblew999/charter/site/intl/config"
)

// addLocale is charter intl add-locale <locale>…: a language, its id as Intl writes it, added to intl.toml with
// its catalog.
func addLocale(args []string) {
	c := app()
	var err error
	if len(args) == 0 {
		fail(errors.New("charter intl add-locale <locale>…: the BCP 47 ids of the languages to add, fr or pt-BR"))
	}
	startIntl()
	for _, id := range args {
		canon, ok := intl.NewLocale("en").Intl(intl.Call{API: "Intl", Method: "getCanonicalLocales", Args: []string{intl.JSON(id)}}, "", "")
		var ids []string
		if !ok || json.Unmarshal([]byte(canon), &ids) != nil || len(ids) != 1 {
			fail(fmt.Errorf("intl: %q is not a BCP 47 locale id", id))
		}
		l := intl.NewLocale(ids[0])
		must(catalog.AddLocale(c, ids[0], l.Name()))
		fmt.Printf("added %s, %s: translate it (charter intl todo %s)\n", ids[0], l.Name(), ids[0])
		c, err = config.Find(".")
		must(err)
	}
	p := read(c)
	must(p.Generate(os.Stderr))
}
