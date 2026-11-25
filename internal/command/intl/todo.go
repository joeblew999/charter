package intl

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joeblew999/charter/internal/intlkit/catalog"
	"github.com/joeblew999/charter/intl"
	"github.com/joeblew999/charter/intl/config"
	"github.com/joeblew999/charter/intl/intltest"
)

// todo is charter intl todo: what is left to translate, with what a translator needs, for people and agents
// (--json).
func todo(args []string) {
	c := app()
	f := flag.NewFlagSet("todo", flag.ExitOnError)
	asJSON := f.Bool("json", false, "JSON on stdout, for agents")
	f.Parse(args)
	todoList(c, read(c), *asJSON, f.Args())
}

// todoLocale is a locale in charter intl todo -json.
type todoLocale struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	File    string     `json:"file"`
	Note    string     `json:"note,omitempty"`
	Plurals todoPlural `json:"plurals"`
	Todo    []todoItem `json:"todo"`
}

type todoPlural struct {
	Cardinal []string `json:"cardinal"`
	Ordinal  []string `json:"ordinal"`
}

type todoItem struct {
	Key         string        `json:"key"`
	State       string        `json:"state"`
	Source      string        `json:"source"`
	Translation string        `json:"translation,omitempty"`
	Context     string        `json:"context,omitempty"`
	Args        []catalog.Arg `json:"args,omitempty"`
}

func todoList(c config.Config, p *catalog.Project, asJSON bool, ids []string) {
	startIntl()
	var out []todoLocale
	for _, l := range pick(p, ids) {
		cardinal, ordinal, err := intltest.Categories(l.ID)
		must(err)
		rel, _ := filepath.Rel(c.Root, l.File)
		tl := todoLocale{ID: l.ID, Name: intl.NewLocale(l.ID).Name(), File: rel, Note: c.Note(l.ID),
			Plurals: todoPlural{cardinal, ordinal}, Todo: []todoItem{}}
		for _, t := range l.Todo(p) {
			tl.Todo = append(tl.Todo, todoItem{Key: t.Key, State: t.State.String(), Source: t.Text, Translation: t.Translation, Context: t.Context, Args: t.Args})
		}
		out = append(out, tl)
	}
	if asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		must(enc.Encode(map[string]any{"source": p.Locales[0].ID, "locales": out}))
		return
	}
	left := 0
	for _, l := range out {
		if len(l.Todo) == 0 {
			continue
		}
		left += len(l.Todo)
		fmt.Printf("%s (%s) %s: %d to translate · plural: %s · selectordinal: %s\n", l.ID, l.Name, l.File, len(l.Todo),
			strings.Join(l.Plurals.Cardinal, " "), strings.Join(l.Plurals.Ordinal, " "))
		if l.Note != "" {
			fmt.Printf("  note: %s\n", l.Note)
		}
		for _, t := range l.Todo {
			fmt.Printf("  %-7s %s = %q\n", t.State, t.Key, t.Source)
			if t.Context != "" {
				fmt.Printf("          context: %s\n", t.Context)
			}
			if t.Translation != "" {
				fmt.Printf("          was translated as %q\n", t.Translation)
			}
		}
	}
	if left == 0 {
		fmt.Println("nothing to translate")
	}
}
