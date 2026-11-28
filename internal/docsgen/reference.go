package docsgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/joeblew999/charter/internal/dbcode"
	"github.com/joeblew999/charter/internal/envcode"
)

// The pages of an app's docs that nobody writes: what the app is, read from where it is declared. Each is
// Markdown, written here and then treated as a page of docs/*.md is, so it is shown, searched, linked and given to
// a language model as the others are. A page is there when its source is.
//
//   - the API, from fern/openapi.json: the spec charter check --fix writes from the contract's Go types;
//   - the database, from db/schema.sql and db/queries.sql;
//   - what the Worker is bound to, from wrangler.jsonc, .dev.vars.example and fnox.toml.
//
// A page a person writes says what an app is for and how to use it. These say what it has, and are never out of
// date: there is nothing to keep in step.

// written is a page charter writes: its place under /docs, its Markdown, and the file it was read from (where
// "edit this page" goes: change that, and the page follows).
type written struct{ slug, markdown, from string }

// reference is the pages charter writes of dir's app.
func reference(dir string) ([]written, error) {
	var pages []written
	for _, one := range []func(string) (*written, error){apiPage, databasePage, bindingsPage} {
		p, err := one(dir)
		if err != nil {
			return nil, err
		}
		if p != nil {
			pages = append(pages, *p)
		}
	}
	return pages, nil
}

// from is the line under a written page's title: where it comes from.
func from(sources string) string {
	return "*Written by charter from " + sources + ": it is what the app has now, and changes when that does.*\n\n"
}

// cell is text as a table's cell holds it: on one line, with no bar of its own.
func cell(s string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(s), " "), "|", `\|`)
}

// schema is as much of an OpenAPI schema as the page says.
type schema struct {
	Ref         string             `json:"$ref"`
	Type        any                `json:"type"`
	Format      string             `json:"format"`
	Description string             `json:"description"`
	Examples    []any              `json:"examples"`
	Default     any                `json:"default"`
	Enum        []any              `json:"enum"`
	Minimum     *float64           `json:"minimum"`
	Maximum     *float64           `json:"maximum"`
	MaxLength   *int               `json:"maxLength"`
	Items       *schema            `json:"items"`
	Properties  map[string]*schema `json:"properties"`
	Required    []string           `json:"required"`
}

// typeOf is a schema's type in a few words: Note, string, integer (int64), a list of Note.
func typeOf(s *schema) string {
	switch {
	case s == nil:
		return ""
	case s.Ref != "":
		name := s.Ref[strings.LastIndex(s.Ref, "/")+1:]
		return "[" + name + "](#" + headingID(name) + ")"
	case s.Items != nil:
		return "list of " + typeOf(s.Items)
	}
	t := fmt.Sprint(s.Type)
	if list, ok := s.Type.([]any); ok {
		parts := make([]string, len(list))
		for i, one := range list {
			parts[i] = fmt.Sprint(one)
		}
		t = strings.Join(parts, " or ")
	}
	if s.Type == nil {
		t = "any"
	}
	if s.Format != "" {
		t += " (" + s.Format + ")"
	}
	return t
}

// limits is what a schema says of its value beyond its type: its default, its bounds, its choices, an example.
func limits(s *schema) string {
	if s == nil {
		return ""
	}
	var said []string
	if s.Description != "" {
		said = append(said, s.Description)
	}
	if s.Default != nil {
		said = append(said, fmt.Sprintf("Default `%v`", s.Default))
	}
	switch {
	case s.Minimum != nil && s.Maximum != nil:
		said = append(said, fmt.Sprintf("From %v to %v", *s.Minimum, *s.Maximum))
	case s.Minimum != nil:
		said = append(said, fmt.Sprintf("At least %v", *s.Minimum))
	case s.Maximum != nil:
		said = append(said, fmt.Sprintf("At most %v", *s.Maximum))
	}
	if s.MaxLength != nil {
		said = append(said, fmt.Sprintf("At most %d characters", *s.MaxLength))
	}
	if len(s.Enum) > 0 {
		choices := make([]string, len(s.Enum))
		for i, e := range s.Enum {
			choices[i] = fmt.Sprintf("`%v`", e)
		}
		said = append(said, "One of "+strings.Join(choices, ", "))
	}
	if len(s.Examples) > 0 {
		said = append(said, fmt.Sprintf("For example `%v`", s.Examples[0]))
	}
	return strings.Join(said, ". ")
}

// fields is an object's fields as a table: those it must have first, in the order the contract requires them,
// then the rest by name. "" for what has none.
func fields(s *schema) string {
	if s == nil || len(s.Properties) == 0 {
		return ""
	}
	var out strings.Builder
	out.WriteString("| Field | Type | |\n|---|---|---|\n")
	names := slices.Clone(s.Required)
	for _, name := range slices.Sorted(mapKeys(s.Properties)) {
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	for _, name := range names {
		f := s.Properties[name]
		if f == nil {
			continue
		}
		about := limits(f)
		if !slices.Contains(s.Required, name) {
			about = strings.TrimSuffix("Optional. "+about, " ")
		}
		fmt.Fprintf(&out, "| `%s` | %s | %s |\n", name, typeOf(f), cell(about))
	}
	return out.String()
}

// apiPage is the API as its spec has it: every operation, who may call it and how often, what it takes and
// answers, and the types. nil for an app with no spec.
func apiPage(dir string) (*written, error) {
	const file = "fern/openapi.json"
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file)))
	if err != nil {
		return nil, nil
	}
	type content map[string]struct {
		Schema *schema `json:"schema"`
	}
	type operation struct {
		ID          string                `json:"operationId"`
		Summary     string                `json:"summary"`
		Description string                `json:"description"`
		Security    []map[string][]string `json:"security"`
		Limit       *struct {
			Binding string `json:"binding"`
			Limit   int    `json:"limit"`
			Period  int    `json:"period"`
		} `json:"x-rate-limit"`
		Parameters []struct {
			Name        string  `json:"name"`
			In          string  `json:"in"`
			Required    bool    `json:"required"`
			Description string  `json:"description"`
			Schema      *schema `json:"schema"`
		} `json:"parameters"`
		Body *struct {
			Content content `json:"content"`
		} `json:"requestBody"`
		Responses map[string]struct {
			Description string  `json:"description"`
			Content     content `json:"content"`
		} `json:"responses"`
	}
	var spec struct {
		Info struct {
			Title, Description, Version string
		} `json:"info"`
		Servers []struct {
			URL string `json:"url"`
		} `json:"servers"`
		Security   []map[string][]string           `json:"security"`
		Paths      map[string]map[string]operation `json:"paths"`
		Components struct {
			Schemas         map[string]*schema `json:"schemas"`
			SecuritySchemes map[string]struct {
				Type, Scheme, Description, In, Name string
			} `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(b, &spec); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	// What an operation needs: the scopes of its Security, or the API's when it says nothing. Each way in asks the
	// same, so the first says it.
	needs := func(op operation, public bool) string {
		security := op.Security
		if security == nil && !public {
			security = spec.Security
		}
		var scopes []string
		for _, alternative := range security {
			for _, s := range alternative {
				for _, scope := range s {
					if !slices.Contains(scopes, scope) {
						scopes = append(scopes, scope)
					}
				}
			}
		}
		switch {
		case len(security) == 0:
			return "anyone"
		case len(scopes) == 0:
			return "anyone signed in"
		}
		for i, s := range scopes {
			scopes[i] = "`" + s + "`"
		}
		return strings.Join(scopes, " and ")
	}
	// What a body is, in a few words, and its schema when it is an object written where it is used (one with a
	// name is under Types).
	body := func(c content) (string, *schema) {
		for _, kind := range slices.Sorted(mapKeys(c)) {
			s := c[kind].Schema
			switch {
			case s == nil:
				return "`" + kind + "`", nil
			case s.Ref == "" && len(s.Properties) > 0:
				return "an object (`" + kind + "`)", s
			}
			return typeOf(s) + " (`" + kind + "`)", nil
		}
		return "", nil
	}
	var out strings.Builder
	title := spec.Info.Title
	if title == "" {
		title = "this app"
	}
	fmt.Fprintf(&out, "# The API of %s\n\n", title)
	out.WriteString("Every operation the API has, who may call it, what it takes and what it answers.\n\n")
	out.WriteString(from("the API's contract (`" + file + "`, which `charter check` holds to the Go types the API is written in)"))
	out.WriteString("The contract itself, for a program to read: [`/api/openapi.json`](/api/openapi.json).")
	if len(spec.Servers) > 0 && spec.Servers[0].URL != "" {
		fmt.Fprintf(&out, " The API is served at `%s`.", spec.Servers[0].URL)
	}
	out.WriteString("\n\n## Operations\n\n| Operation | What it does | Who may call it | How often |\n|---|---|---|---|\n")
	type listed struct {
		method, path string
		op           operation
	}
	var all []listed
	for _, path := range slices.Sorted(mapKeys(spec.Paths)) {
		for _, method := range []string{"get", "post", "put", "patch", "delete"} {
			if op, ok := spec.Paths[path][method]; ok {
				all = append(all, listed{strings.ToUpper(method), path, op})
			}
		}
	}
	anchor := func(l listed) string { return headingID(l.method + " " + l.path) }
	for _, l := range all {
		often := "no limit of its own"
		if l.op.Limit != nil {
			often = fmt.Sprintf("%d calls in %d s, for each caller", l.op.Limit.Limit, l.op.Limit.Period)
		}
		fmt.Fprintf(&out, "| [`%s %s`](#%s) | %s | %s | %s |\n", l.method, l.path, anchor(l), cell(l.op.Summary), needs(l.op, l.op.Security != nil && len(l.op.Security) == 0), often)
	}
	if len(spec.Components.SecuritySchemes) > 0 {
		out.WriteString("\n## Ways in\n\nA caller says who it is in one of these ways. What it may then do is what it was granted: the scopes above.\n\n| Way | How it is sent |\n|---|---|\n")
		for _, name := range slices.Sorted(mapKeys(spec.Components.SecuritySchemes)) {
			s := spec.Components.SecuritySchemes[name]
			how := s.Description
			switch {
			case how != "":
			case s.Type == "http":
				how = "`Authorization: " + capitalised(s.Scheme) + " <token>`"
			case s.Type == "apiKey":
				how = "The " + s.In + " `" + s.Name + "`"
			}
			fmt.Fprintf(&out, "| `%s` | %s |\n", name, cell(how))
		}
	}
	for _, l := range all {
		fmt.Fprintf(&out, "\n## %s %s\n\n", l.method, l.path)
		if l.op.Summary != "" {
			out.WriteString(strings.TrimRight(l.op.Summary, ".") + ".")
		}
		if l.op.Description != "" && l.op.Description != l.op.Summary {
			out.WriteString(" " + l.op.Description)
		}
		fmt.Fprintf(&out, " In the SDKs and the specs it is `%s`.\n\n", l.op.ID)
		if len(l.op.Parameters) > 0 {
			out.WriteString("| It takes | Where | Type | |\n|---|---|---|---|\n")
			for _, p := range l.op.Parameters {
				about := limits(p.Schema)
				if p.Schema != nil && p.Schema.Description == "" && p.Description != "" {
					about = strings.TrimPrefix(p.Description+". "+about, ". ")
				}
				if p.Required {
					about = strings.TrimSuffix("Required. "+about, " ")
				}
				fmt.Fprintf(&out, "| `%s` | %s | %s | %s |\n", p.Name, p.In, typeOf(p.Schema), cell(about))
			}
			out.WriteString("\n")
		}
		if l.op.Body != nil {
			if b, inline := body(l.op.Body.Content); b != "" {
				fmt.Fprintf(&out, "Its body is %s", b)
				if inline != nil {
					out.WriteString(":\n\n" + fields(inline) + "\n")
				} else {
					out.WriteString(".\n\n")
				}
			}
		}
		out.WriteString("| It answers | With |\n|---|---|\n")
		var answered *schema
		for _, status := range slices.Sorted(mapKeys(l.op.Responses)) {
			r := l.op.Responses[status]
			with, inline := body(r.Content)
			if with == "" {
				with = "nothing"
			}
			if inline != nil && strings.HasPrefix(status, "2") {
				answered = inline
				with += ", below"
			}
			fmt.Fprintf(&out, "| %s %s | %s |\n", status, cell(r.Description), with)
		}
		if answered != nil {
			out.WriteString("\nWhat it answers with:\n\n" + fields(answered))
		}
	}
	if len(spec.Components.Schemas) > 0 {
		out.WriteString("\n## Types\n")
		for _, name := range slices.Sorted(mapKeys(spec.Components.Schemas)) {
			s := spec.Components.Schemas[name]
			fmt.Fprintf(&out, "\n### %s\n\n", name)
			if s.Description != "" {
				out.WriteString(s.Description + "\n\n")
			}
			out.WriteString(fields(s))
		}
	}
	return &written{"reference-api", out.String(), file}, nil
}

// headingID is the id goldmark gives a heading with this text (its WithAutoHeadingID): letters and digits in lower
// case, a space as a hyphen, everything else left out. The audit follows the links made with it.
func headingID(title string) string {
	var id strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
			id.WriteRune(r)
		case r == ' ':
			id.WriteByte('-')
		}
	}
	return id.String()
}

func capitalised(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func mapKeys[V any](m map[string]V) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// query is one query of db/queries.sql: its name, what it returns (:one, :many, :exec, :execrows), what its
// comment says of it, and its SQL.
type query struct{ name, returns, about, sql string }

// queries reads db/queries.sql as sqlc does: "-- name: Note :one", the comment lines after it, then the statement.
func queries(text string) []query {
	var out []query
	named := regexp.MustCompile(`^--\s*name:\s*(\w+)\s+(:\w+)`)
	var q *query
	for _, line := range strings.Split(text, "\n") {
		if m := named.FindStringSubmatch(line); m != nil {
			out = append(out, query{name: m[1], returns: m[2]})
			q = &out[len(out)-1]
			continue
		}
		if q == nil {
			continue
		}
		if comment, ok := strings.CutPrefix(strings.TrimSpace(line), "--"); ok {
			if q.sql == "" {
				q.about = strings.TrimSpace(q.about + " " + strings.TrimSpace(comment))
			}
			continue
		}
		if strings.TrimSpace(line) != "" {
			q.sql = strings.TrimSpace(q.sql + "\n" + strings.TrimRight(line, " \t"))
		}
	}
	return out
}

// databasePage is the database as its schema has it: each table with its columns, and each query the app runs on
// it. nil for an app with no schema.
func databasePage(dir string) (*written, error) {
	const file = "db/schema.sql"
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file)))
	if err != nil {
		return nil, nil
	}
	tables, err := dbcode.Tables(string(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	var out strings.Builder
	out.WriteString("# The database\n\nEvery table the app keeps, and every statement it runs on them.\n\n")
	out.WriteString(from("`db/schema.sql` and `db/queries.sql`, the one place a table and a statement are written down"))
	out.WriteString("## Tables\n")
	for _, t := range tables {
		fmt.Fprintf(&out, "\n### %s\n\n| Column | Type | | |\n|---|---|---|---|\n", t.Name)
		for _, c := range t.Columns {
			var is []string
			if c.Key {
				is = append(is, "the key")
			} else if !c.NotNull {
				is = append(is, "may be empty (NULL)")
			}
			if c.Default != "" {
				is = append(is, "default `"+c.Default+"`")
			}
			kind := c.Type
			if kind == "" {
				kind = "any"
			}
			fmt.Fprintf(&out, "| `%s` | %s | %s | %s |\n", c.Name, kind, cell(strings.Join(is, ", ")), cell(c.Doc))
		}
	}
	text, _ := os.ReadFile(filepath.Join(dir, "db", "queries.sql"))
	if all := queries(string(text)); len(all) > 0 {
		out.WriteString("\n## Statements\n\nEach is a function of the package `db`, with its arguments and its rows typed against the tables above. No statement is written anywhere else.\n\n| Function | It gives | What it does |\n|---|---|---|\n")
		gives := map[string]string{":one": "one row, or no such row", ":many": "rows", ":exec": "nothing", ":execrows": "how many rows it changed"}
		for _, q := range all {
			// What its comment says of it, as it is written; with none, the statement itself.
			about := q.about
			if about == "" {
				about = "`" + strings.TrimSuffix(strings.Join(strings.Fields(q.sql), " "), ";") + "`"
			}
			fmt.Fprintf(&out, "| `db.%s` | %s | %s |\n", q.name, gives[q.returns], cell(about))
		}
	}
	return &written{"reference-database", out.String(), file}, nil
}

// bindingsPage is what the Worker is bound to, as its files declare it. nil for a Worker that declares nothing.
func bindingsPage(dir string) (*written, error) {
	if _, err := os.Stat(filepath.Join(dir, "wrangler.jsonc")); err != nil {
		return nil, nil
	}
	declared, err := envcode.Read(dir)
	if err != nil || len(declared) == 0 {
		return nil, err
	}
	var out strings.Builder
	out.WriteString("# What the Worker is bound to\n\nEverything Cloudflare gives this Worker by name: its databases, buckets and queues, its variables, and the names of its secrets.\n\n")
	out.WriteString(from("`wrangler.jsonc`, `.dev.vars.example` and `fnox.toml`"))
	out.WriteString("Each is a value in the app's Go under the same name (`env.DB`, `env.APP_NAME`), so one that is not here cannot be used. A secret's value is never in the repository, and never here.\n\n| | In Go |\n|---|---|\n")
	for _, d := range declared {
		names := make([]string, len(d.Names))
		for i, name := range d.Names {
			names[i] = "`env." + name + "`"
		}
		fmt.Fprintf(&out, "| %s | %s |\n", cell(d.About), strings.Join(names, ", "))
	}
	return &written{"reference-bindings", out.String(), "wrangler.jsonc"}, nil
}
