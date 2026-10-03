package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// `charter spec-diff` compares the project's OpenAPI and AsyncAPI specs with those of its previous
// release and says which changes break a consumer: a generated SDK, a CLI, a client written by
// hand. It fails on a breaking change, unless the release it is run for (-tag) is a major one. With
// a catalog (charter catalog -json) it names the repos that pin this one.
//
// Directions matter: a consumer sends parameters and request bodies, and receives responses and
// the messages of an AsyncAPI "receive" operation (the specs are written from the client's side).
// What a consumer sends may widen and what it receives may narrow; the reverse breaks it.

func init() {
	commands["spec-diff"] = command{"[-from vX.Y.Z] [-to vX.Y.Z] [-tag vX.Y.Z] [-catalog <file or url>]",
		"compare fern/openapi.json and fern/asyncapi.json with the previous release's (git tags) and fail on a breaking change unless -tag is a major release; -catalog names the consumers it affects", specDiff}
}

// specChange is one difference between two specs.
type specChange struct {
	Breaking bool
	Where    string // GET /api/notes, channel liveNotes
	What     string
}

var specFiles = []string{"fern/openapi.json", "fern/asyncapi.json"}

func specDiff(args []string) error {
	var from, to, tag, catalogFrom string
	flags("spec-diff", args, func(f *flag.FlagSet) {
		f.StringVar(&from, "from", "", "the release to compare with (default: the newest version tag before this one)")
		f.StringVar(&to, "to", "", "a version tag to compare (default: the working tree)")
		f.StringVar(&tag, "tag", "", "the release being made (default: the tag a workflow runs on): a breaking change passes only if it is a major one")
		f.StringVar(&catalogFrom, "catalog", "", "what charter catalog -json wrote (a file or a URL): name the repos that pin this one")
	})
	if tag == "" && os.Getenv("GITHUB_REF_TYPE") == "tag" {
		tag = os.Getenv("GITHUB_REF_NAME")
	}
	if tag != "" && !version.MatchString(tag) {
		return fmt.Errorf("-tag %q is not a version tag (v1.2.3)", tag)
	}
	prefix, err := output(".", "git", "rev-parse", "--show-prefix")
	if err != nil {
		return errors.New("spec-diff reads the previous release from git: run it in a git checkout")
	}
	if from == "" {
		if from, err = previousRelease(tag); err != nil {
			return err
		}
		if from == "" {
			fmt.Println("no earlier version tag: nothing to compare with")
			return nil
		}
	}
	read := func(ref, file string) ([]byte, error) {
		if ref == "" {
			content, err := os.ReadFile(filepath.FromSlash(file))
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil
			}
			return content, err
		}
		content, err := exec.Command("git", "show", ref+":"+prefix+file).Output()
		if err != nil {
			return nil, nil // not in that release
		}
		return content, nil
	}
	label := to
	if label == "" {
		label = "the working tree"
	}
	fmt.Printf("specs: %s against %s\n", label, from)
	var changes []specChange
	for _, file := range specFiles {
		before, err := read(from, file)
		if err != nil {
			return err
		}
		after, err := read(to, file)
		if err != nil {
			return err
		}
		switch {
		case before == nil && after == nil:
			continue
		case before == nil:
			changes = append(changes, specChange{false, file, "new"})
			continue
		case after == nil:
			changes = append(changes, specChange{true, file, "removed"})
			continue
		}
		found, err := diffSpecs(before, after)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		changes = append(changes, found...)
	}
	breaking := 0
	for _, change := range changes {
		mark := "  "
		if change.Breaking {
			mark, breaking = "! ", breaking+1
		}
		fmt.Printf("%s%s: %s\n", mark, change.Where, change.What)
	}
	fmt.Printf("%d changes, %d breaking (marked !)\n", len(changes), breaking)
	if breaking == 0 {
		return nil
	}
	if catalogFrom != "" {
		if err := printAffected(catalogFrom); err != nil {
			return err
		}
	}
	if tag != "" && majorRelease(from, tag) {
		fmt.Printf("%s is a major release after %s: breaking changes are allowed\n", tag, from)
		return nil
	}
	return fmt.Errorf("%d breaking changes since %s: they need a major release (in v0, the minor number)", breaking, from)
}

// previousRelease is the newest version tag in the history of HEAD other than tag, the release
// being made.
func previousRelease(tag string) (string, error) {
	list, err := output(".", "git", "tag", "--merged", "HEAD", "--list", "v*", "--sort=-v:refname")
	if err != nil {
		return "", err
	}
	for _, candidate := range strings.Fields(list) {
		if candidate != tag && version.MatchString(candidate) {
			return candidate, nil
		}
	}
	return "", nil
}

// majorRelease reports whether next breaks compatibility with previous by semantic versioning: a
// higher major number, or in v0 a higher minor one.
func majorRelease(previous, next string) bool {
	p, n := version.FindStringSubmatch(previous), version.FindStringSubmatch(next)
	if p == nil || n == nil {
		return false
	}
	number := func(s string) int { v, _ := strconv.Atoi(s); return v }
	if number(n[1]) != number(p[1]) {
		return number(n[1]) > number(p[1])
	}
	return number(n[1]) == 0 && number(n[2]) > number(p[2])
}

// printAffected names the repos that pin this one's API: its binaries, Go modules or npm packages
// (a pin of its task folders is not touched by the specs).
func printAffected(from string) error {
	var content []byte
	var err error
	if strings.HasPrefix(from, "https://") || strings.HasPrefix(from, "http://") {
		var response *http.Response
		if response, err = http.Get(from); err == nil {
			defer response.Body.Close()
			content, err = io.ReadAll(response.Body)
		}
	} else {
		if !filepath.IsAbs(from) {
			from = filepath.Join(started, from)
		}
		content, err = os.ReadFile(from)
	}
	if err != nil {
		return err
	}
	var found catalog
	if err := json.Unmarshal(content, &found); err != nil {
		return fmt.Errorf("%s: not what charter catalog -json writes: %w", from, err)
	}
	origin, err := output(".", "git", "remote", "get-url", "origin")
	if err != nil {
		return err
	}
	self := repoOf(origin)
	for _, repo := range found.Repos {
		if !strings.EqualFold(repo.Repo, self) {
			continue
		}
		affected := 0
		for _, pin := range repo.Consumers {
			if pin.Kind != "tasks" {
				affected++
				fmt.Printf("affected: %s (%s pins %s %s)\n", pin.Repo, pin.File, pin.Name, pin.Version)
			}
		}
		if affected == 0 {
			fmt.Printf("affected: no repo in the catalog pins %s's API\n", self)
		}
		return nil
	}
	fmt.Printf("affected: %s is not in the catalog\n", self)
	return nil
}

// repoOf is owner/name of a GitHub URL (https or ssh), or "".
func repoOf(url string) string {
	m := githubRepoRef.FindStringSubmatch(url)
	if m == nil {
		return ""
	}
	return m[1] + "/" + m[2]
}

// diffSpecs compares two specs of the same kind, OpenAPI or AsyncAPI.
func diffSpecs(before, after []byte) ([]specChange, error) {
	var a, b map[string]any
	if err := json.Unmarshal(before, &a); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(after, &b); err != nil {
		return nil, err
	}
	d := &specDiffer{a: a, b: b, seen: map[string]bool{}}
	switch {
	case a["openapi"] != nil && b["openapi"] != nil:
		d.openAPI()
	case a["asyncapi"] != nil && b["asyncapi"] != nil:
		d.asyncAPI()
	default:
		return nil, errors.New("not two OpenAPI or two AsyncAPI documents")
	}
	return d.changes, nil
}

type specDiffer struct {
	a, b    map[string]any // the old and the new document, for their $refs
	seen    map[string]bool
	changes []specChange
}

func (d *specDiffer) add(breaking bool, where, format string, args ...any) {
	d.changes = append(d.changes, specChange{breaking, where, fmt.Sprintf(format, args...)})
}

// Which way a value goes: a consumer sends it (request) or receives it (response).
type flow bool

const (
	sends    flow = true
	receives flow = false
)

func object(v any) map[string]any { m, _ := v.(map[string]any); return m }

// resolve follows a local $ref (#/components/schemas/Note) in doc, and the ones that leads to.
func resolve(doc map[string]any, v any) (map[string]any, string) {
	m, ref := object(v), ""
	for i := 0; i < 16 && m != nil; i++ {
		r, ok := m["$ref"].(string)
		if !ok || !strings.HasPrefix(r, "#/") {
			break
		}
		ref = r
		var at any = doc
		for _, part := range strings.Split(strings.TrimPrefix(r, "#/"), "/") {
			at = object(at)[strings.NewReplacer("~1", "/", "~0", "~").Replace(part)]
		}
		m = object(at)
	}
	return m, ref
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

func stringsOf(v any) []string {
	var out []string
	for _, item := range anyList(v) {
		if s, ok := item.(string); ok {
			out = append(out, s)
		} else {
			raw, _ := json.Marshal(item)
			out = append(out, string(raw))
		}
	}
	return out
}

func anyList(v any) []any { l, _ := v.([]any); return l }

// types are a schema's types; "type": ["string", "null"] in OpenAPI 3.1.
func types(schema map[string]any) []string {
	switch t := schema["type"].(type) {
	case string:
		return []string{t}
	case []any:
		list := stringsOf(t)
		slices.Sort(list)
		return list
	}
	return nil
}

func subset(small, big []string) bool {
	for _, s := range small {
		if !slices.Contains(big, s) {
			return false
		}
	}
	return true
}

// schema compares a schema of the old document with its counterpart in the new.
func (d *specDiffer) schema(where, path string, way flow, before, after any) {
	a, refA := resolve(d.a, before)
	b, refB := resolve(d.b, after)
	if a == nil || b == nil {
		return
	}
	if refA != "" && refB != "" { // a recursive schema is compared once per place and direction
		key := fmt.Sprint(where, refA, refB, way)
		if d.seen[key] {
			return
		}
		d.seen[key] = true
	}
	field := func() string {
		if path == "" {
			return "the message"
		}
		return path
	}
	ta, tb := types(a), types(b)
	if len(ta) > 0 && len(tb) > 0 && !slices.Equal(ta, tb) {
		widened := way == sends && subset(ta, tb) || way == receives && subset(tb, ta)
		d.add(!widened, where, "%s: type %s, was %s", field(), strings.Join(tb, " or "), strings.Join(ta, " or "))
	}
	if ea, eb := stringsOf(a["enum"]), stringsOf(b["enum"]); len(ea) > 0 || len(eb) > 0 {
		for _, value := range ea {
			if len(eb) > 0 && !slices.Contains(eb, value) {
				d.add(true, where, "%s: enum value %s removed", field(), value)
			}
		}
		for _, value := range eb {
			if len(ea) > 0 && !slices.Contains(ea, value) {
				d.add(false, where, "%s: enum value %s added", field(), value)
			}
		}
		if len(ea) == 0 && way == sends {
			d.add(true, where, "%s: now limited to %s", field(), strings.Join(eb, ", "))
		}
	}
	pa, pb := object(a["properties"]), object(b["properties"])
	ra, rb := stringsOf(a["required"]), stringsOf(b["required"])
	join := func(name string) string {
		if path == "" {
			return name
		}
		return path + "." + name
	}
	for _, name := range sortedKeys(pa) {
		if _, ok := pb[name]; !ok {
			d.add(true, where, "field %s removed", join(name))
			continue
		}
		wasRequired, isRequired := slices.Contains(ra, name), slices.Contains(rb, name)
		switch {
		case way == sends && !wasRequired && isRequired:
			d.add(true, where, "field %s now required", join(name))
		case way == receives && wasRequired && !isRequired:
			d.add(true, where, "field %s no longer always there", join(name))
		}
		d.schema(where, join(name), way, pa[name], pb[name])
	}
	for _, name := range sortedKeys(pb) {
		if _, ok := pa[name]; ok {
			continue
		}
		if way == sends && slices.Contains(rb, name) {
			d.add(true, where, "required field %s added", join(name))
		} else {
			d.add(false, where, "field %s added", join(name))
		}
	}
	if a["items"] != nil && b["items"] != nil {
		d.schema(where, path+"[]", way, a["items"], b["items"])
	}
	if object(a["additionalProperties"]) != nil && object(b["additionalProperties"]) != nil {
		d.schema(where, path+"{}", way, a["additionalProperties"], b["additionalProperties"])
	}
}

var httpMethods = []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}

// operations are a document's operations by "METHOD /path".
func operations(doc map[string]any) (map[string]map[string]any, map[string]map[string]any) {
	ops, items := map[string]map[string]any{}, map[string]map[string]any{}
	for path, v := range object(doc["paths"]) {
		item, _ := resolve(doc, v)
		for _, method := range httpMethods {
			if op := object(item[method]); op != nil {
				key := strings.ToUpper(method) + " " + path
				ops[key], items[key] = op, item
			}
		}
	}
	return ops, items
}

// sdkName is the name a generated SDK gives an operation: Fern's group and method, or the
// operationId.
func sdkName(op map[string]any) string {
	group, _ := op["x-fern-sdk-group-name"].(string)
	method, _ := op["x-fern-sdk-method-name"].(string)
	if method != "" {
		return strings.TrimPrefix(group+"."+method, ".")
	}
	id, _ := op["operationId"].(string)
	return id
}

func (d *specDiffer) openAPI() {
	before, itemsA := operations(d.a)
	after, itemsB := operations(d.b)
	for _, key := range sortedKeys(anyMap(before)) {
		a, b := before[key], after[key]
		if b == nil {
			moved := ""
			for other, op := range after {
				if before[other] == nil && sdkName(op) == sdkName(a) && sdkName(a) != "" {
					moved = ", now " + other
				}
			}
			d.add(true, key, "operation %s removed%s", sdkName(a), moved)
			continue
		}
		if na, nb := sdkName(a), sdkName(b); na != nb {
			d.add(true, key, "SDK method %s, was %s", nb, na)
		}
		d.parameters(key, itemsA[key], a, itemsB[key], b)
		d.requestBody(key, a, b)
		d.responses(key, a, b)
		d.security(key, a, b)
	}
	for _, key := range sortedKeys(anyMap(after)) {
		if before[key] == nil {
			d.add(false, key, "operation %s added", sdkName(after[key]))
		}
	}
}

func anyMap[V any](m map[string]V) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

// parameters are an operation's parameters, with its path's, by "in name".
func parameters(doc, item, op map[string]any) map[string]map[string]any {
	found := map[string]map[string]any{}
	for _, list := range []any{item["parameters"], op["parameters"]} {
		for _, v := range anyList(list) {
			p, _ := resolve(doc, v)
			if p != nil {
				found[fmt.Sprint(p["in"], " ", p["name"])] = p
			}
		}
	}
	return found
}

func (d *specDiffer) parameters(where string, itemA, a, itemB, b map[string]any) {
	pa, pb := parameters(d.a, itemA, a), parameters(d.b, itemB, b)
	for _, key := range sortedKeys(anyMap(pa)) {
		p, q := pa[key], pb[key]
		if q == nil {
			d.add(true, where, "parameter %s removed", key)
			continue
		}
		if p["required"] != true && q["required"] == true {
			d.add(true, where, "parameter %s now required", key)
		}
		d.schema(where, strings.ReplaceAll(key, " ", "."), sends, p["schema"], q["schema"])
	}
	for _, key := range sortedKeys(anyMap(pb)) {
		if pa[key] == nil {
			d.add(pb[key]["required"] == true, where, "parameter %s added%s", key, map[bool]string{true: ", required"}[pb[key]["required"] == true])
		}
	}
}

// jsonSchema is the schema of a body's JSON content, or of its first content type.
func jsonSchema(content map[string]any) any {
	for _, kind := range sortedKeys(content) {
		if strings.Contains(kind, "json") {
			return object(content[kind])["schema"]
		}
	}
	for _, kind := range sortedKeys(content) {
		return object(content[kind])["schema"]
	}
	return nil
}

func (d *specDiffer) requestBody(where string, a, b map[string]any) {
	ba, _ := resolve(d.a, a["requestBody"])
	bb, _ := resolve(d.b, b["requestBody"])
	switch {
	case ba == nil && bb == nil:
	case ba == nil:
		d.add(bb["required"] == true, where, "request body added")
	case bb == nil:
		d.add(true, where, "request body removed")
	default:
		if ba["required"] != true && bb["required"] == true {
			d.add(true, where, "request body now required")
		}
		d.schema(where, "request", sends, jsonSchema(object(ba["content"])), jsonSchema(object(bb["content"])))
	}
}

func (d *specDiffer) responses(where string, a, b map[string]any) {
	ra, rb := object(a["responses"]), object(b["responses"])
	for _, code := range sortedKeys(ra) {
		if !strings.HasPrefix(code, "2") {
			continue
		}
		if rb[code] == nil {
			d.add(true, where, "response %s removed", code)
			continue
		}
		x, _ := resolve(d.a, ra[code])
		y, _ := resolve(d.b, rb[code])
		d.schema(where, "response", receives, jsonSchema(object(x["content"])), jsonSchema(object(y["content"])))
	}
}

// requirements are the ways to be let in: one map of scheme to scopes each, any one of which is
// enough. No requirement, or an empty one, lets anyone in.
func requirements(doc, op map[string]any) []map[string][]string {
	list, ok := op["security"]
	if !ok {
		list = doc["security"]
	}
	var ways []map[string][]string
	for _, v := range anyList(list) {
		way := map[string][]string{}
		for scheme, scopes := range object(v) {
			way[scheme] = stringsOf(scopes)
		}
		ways = append(ways, way)
	}
	if len(ways) == 0 {
		ways = append(ways, map[string][]string{})
	}
	return ways
}

func describeWays(ways []map[string][]string) string {
	var out []string
	for _, way := range ways {
		var parts []string
		for _, scheme := range sortedKeys(anyMap(way)) {
			parts = append(parts, scheme+strings.ReplaceAll(fmt.Sprint(way[scheme]), " ", ","))
		}
		if len(parts) == 0 {
			parts = []string{"anyone"}
		}
		out = append(out, strings.Join(parts, "+"))
	}
	return strings.Join(out, " or ")
}

// security fails when a caller let in before is not let in now: a new way in must ask for no
// scheme and no scope that some old way did not.
func (d *specDiffer) security(where string, a, b map[string]any) {
	before, after := requirements(d.a, a), requirements(d.b, b)
	enough := func(have, need map[string][]string) bool {
		for scheme, scopes := range need {
			got, ok := have[scheme]
			if !ok || !subset(scopes, got) {
				return false
			}
		}
		return true
	}
	for _, old := range before {
		if !slices.ContainsFunc(after, func(way map[string][]string) bool { return enough(old, way) }) {
			d.add(true, where, "security tightened: %s, was %s", describeWays(after), describeWays(before))
			return
		}
	}
	if describeWays(before) != describeWays(after) {
		d.add(false, where, "security loosened: %s, was %s", describeWays(after), describeWays(before))
	}
}

// asyncAPI compares AsyncAPI 3 documents: channels (address, messages, a WebSocket's query) and
// operations.
func (d *specDiffer) asyncAPI() {
	// A message flows as the operations on its channel say: "receive" is the consumer receiving.
	ways := func(doc map[string]any) map[string]flow {
		found := map[string]flow{}
		for _, v := range object(doc["operations"]) {
			op := object(v)
			if channel, ok := object(op["channel"])["$ref"].(string); ok {
				found[strings.TrimPrefix(channel, "#/channels/")] = flow(op["action"] == "send")
			}
		}
		return found
	}
	flows := ways(d.b)
	ca, cb := object(d.a["channels"]), object(d.b["channels"])
	for _, name := range sortedKeys(ca) {
		where := "channel " + name
		a, _ := resolve(d.a, ca[name])
		b, _ := resolve(d.b, cb[name])
		if b == nil {
			d.add(true, where, "removed")
			continue
		}
		if a["address"] != b["address"] {
			d.add(true, where, "address %v, was %v", b["address"], a["address"])
		}
		ma, mb := object(a["messages"]), object(b["messages"])
		for _, message := range sortedKeys(ma) {
			if mb[message] == nil {
				d.add(true, where, "message %s removed", message)
				continue
			}
			x, _ := resolve(d.a, ma[message])
			y, _ := resolve(d.b, mb[message])
			d.schema(where, message, flows[name], x["payload"], y["payload"])
		}
		for _, message := range sortedKeys(mb) {
			if ma[message] == nil {
				d.add(false, where, "message %s added", message)
			}
		}
		for _, binding := range sortedKeys(object(a["bindings"])) {
			d.schema(where, binding+".query", sends, object(object(a["bindings"])[binding])["query"], object(object(b["bindings"])[binding])["query"])
		}
	}
	for _, name := range sortedKeys(cb) {
		if ca[name] == nil {
			d.add(false, "channel "+name, "added")
		}
	}
	oa, ob := object(d.a["operations"]), object(d.b["operations"])
	for _, name := range sortedKeys(oa) {
		a, b := object(oa[name]), object(ob[name])
		switch {
		case b == nil:
			d.add(true, "operation "+name, "removed")
		case a["action"] != b["action"]:
			d.add(true, "operation "+name, "action %v, was %v", b["action"], a["action"])
		}
	}
	for _, name := range sortedKeys(ob) {
		if oa[name] == nil {
			d.add(false, "operation "+name, "added")
		}
	}
}
