package catalog

import (
	"fmt"
	"strconv"
	"strings"
)

// argKind is how a message uses an argument.
type argKind uint8

const (
	argString argKind = iota // {name}, select
	argNumber                // number, plural, selectordinal
)

// parsed is one message parsed with argument names (indexes are assigned later from the source locale).
type parsed struct {
	parts []node
	args  map[string]argKind
	order []string // argument names in first-use order
}

// kind is what a message part is.
type kind uint8

const (
	kText          kind = iota // literal text
	kArg                       // {name}: a string, or a number in the default format
	kNumber                    // {name, number[, style]}
	kPlural                    // {name, plural, [offset:N] =0 {…} one {…} other {…}}
	kSelectOrdinal             // {name, selectordinal, one {…} other {…}}
	kSelect                    // {name, select, male {…} other {…}}
	kPound                     // # inside plural/selectordinal: the number (minus offset)
)

// node is one part of a parsed message, its argument by name.
type node struct {
	kind   kind
	text   string
	arg    string
	style  string
	offset int32
	cases  []caseNode
}

type caseNode struct {
	key   string
	parts []node
}

// parse parses ICU MessageFormat 1 with ICU's apostrophe rules: ” is a literal apostrophe, and an
// apostrophe before { } # (or |) starts quoted literal text up to the next single apostrophe.
func parse(src string) (*parsed, error) {
	p := &msgParser{s: []rune(src), res: &parsed{args: map[string]argKind{}}}
	parts, err := p.parts(0, false)
	if err != nil {
		return nil, err
	}
	if p.i != len(p.s) {
		return nil, fmt.Errorf("unexpected %q at %d", string(p.s[p.i]), p.i)
	}
	p.res.parts = parts
	return p.res, nil
}

type msgParser struct {
	s   []rune
	i   int
	res *parsed
}

// parts reads message text until the end or an unmatched '}' (inside a case). inPlural enables '#'.
func (p *msgParser) parts(depth int, inPlural bool) ([]node, error) {
	var out []node
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			out = append(out, node{kind: kText, text: text.String()})
			text.Reset()
		}
	}
	for p.i < len(p.s) {
		c := p.s[p.i]
		switch {
		case c == '\'':
			if p.i+1 < len(p.s) && p.s[p.i+1] == '\'' {
				text.WriteRune('\'')
				p.i += 2
				continue
			}
			if p.i+1 < len(p.s) && strings.ContainsRune("{}#|", p.s[p.i+1]) {
				p.i++
				for p.i < len(p.s) {
					if p.s[p.i] == '\'' {
						if p.i+1 < len(p.s) && p.s[p.i+1] == '\'' {
							text.WriteRune('\'')
							p.i += 2
							continue
						}
						p.i++
						break
					}
					text.WriteRune(p.s[p.i])
					p.i++
				}
				continue
			}
			text.WriteRune(c)
			p.i++
		case c == '{':
			flush()
			n, err := p.argument(depth, inPlural)
			if err != nil {
				return nil, err
			}
			out = append(out, n)
		case c == '}':
			if depth == 0 {
				return nil, fmt.Errorf("unmatched } at %d", p.i)
			}
			flush()
			return out, nil
		case c == '#' && inPlural:
			flush()
			out = append(out, node{kind: kPound})
			p.i++
		default:
			text.WriteRune(c)
			p.i++
		}
	}
	if depth > 0 {
		return nil, fmt.Errorf("unclosed {")
	}
	flush()
	return out, nil
}

func (p *msgParser) skipSpace() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\t' || p.s[p.i] == '\n' || p.s[p.i] == '\r') {
		p.i++
	}
}

func (p *msgParser) word() string {
	p.skipSpace()
	start := p.i
	for p.i < len(p.s) && !strings.ContainsRune(" \t\n\r,{}", p.s[p.i]) {
		p.i++
	}
	return string(p.s[start:p.i])
}

// argument reads {name}, {name, type[, style | cases]}.
func (p *msgParser) argument(depth int, inPlural bool) (node, error) {
	p.i++ // {
	name := p.word()
	if name == "" {
		return node{}, fmt.Errorf("empty argument name at %d", p.i)
	}
	p.skipSpace()
	use := func(k argKind) {
		if _, seen := p.res.args[name]; !seen {
			p.res.order = append(p.res.order, name)
			p.res.args[name] = k
		} else if k == argNumber {
			p.res.args[name] = argNumber
		}
	}
	if p.i < len(p.s) && p.s[p.i] == '}' {
		p.i++
		use(argString)
		return node{kind: kArg, arg: name}, nil
	}
	if p.i >= len(p.s) || p.s[p.i] != ',' {
		return node{}, fmt.Errorf("argument %s: expected , or }", name)
	}
	p.i++
	typ := p.word()
	p.skipSpace()
	switch typ {
	case "number":
		use(argNumber)
		style := ""
		if p.i < len(p.s) && p.s[p.i] == ',' {
			p.i++
			p.skipSpace()
			start := p.i
			for p.i < len(p.s) && p.s[p.i] != '}' {
				p.i++
			}
			style = strings.TrimSpace(string(p.s[start:p.i]))
		}
		if p.i >= len(p.s) || p.s[p.i] != '}' {
			return node{}, fmt.Errorf("argument %s: unclosed number", name)
		}
		if style != "" && style != "integer" {
			// Messages format plain numbers only; currency, percent, units and compact are Intl rows of their own
			// (<data data-intl="number">).
			return node{}, fmt.Errorf("argument %s: number style %q: only the default and \"integer\" are supported", name, style)
		}
		p.i++
		return node{kind: kNumber, arg: name, style: style}, nil
	case "plural", "selectordinal", "select":
		k := map[string]kind{"plural": kPlural, "selectordinal": kSelectOrdinal, "select": kSelect}[typ]
		if typ == "select" {
			use(argString)
		} else {
			use(argNumber)
		}
		if p.i >= len(p.s) || p.s[p.i] != ',' {
			return node{}, fmt.Errorf("argument %s: %s needs cases", name, typ)
		}
		p.i++
		n := node{kind: k, arg: name}
		for {
			p.skipSpace()
			if p.i < len(p.s) && p.s[p.i] == '}' {
				p.i++
				break
			}
			key := p.word()
			if strings.HasPrefix(key, "offset:") && k == kPlural {
				v, err := strconv.Atoi(strings.TrimPrefix(key, "offset:"))
				if err != nil {
					return node{}, fmt.Errorf("argument %s: bad %s", name, key)
				}
				n.offset = int32(v)
				continue
			}
			if key == "" {
				return node{}, fmt.Errorf("argument %s: expected a case key at %d", name, p.i)
			}
			p.skipSpace()
			if p.i >= len(p.s) || p.s[p.i] != '{' {
				return node{}, fmt.Errorf("argument %s: case %s needs {…}", name, key)
			}
			p.i++
			parts, err := p.parts(depth+1, k != kSelect || inPlural)
			if err != nil {
				return node{}, fmt.Errorf("argument %s case %s: %w", name, key, err)
			}
			p.i++ // }
			n.cases = append(n.cases, caseNode{key: key, parts: parts})
		}
		hasOther := false
		for _, c := range n.cases {
			hasOther = hasOther || c.key == "other"
		}
		if !hasOther {
			return node{}, fmt.Errorf("argument %s: %s needs an other case", name, typ)
		}
		return n, nil
	case "date", "time":
		return node{}, fmt.Errorf("argument %s: %s arguments are not supported yet (use a formatted string argument)", name, typ)
	}
	return node{}, fmt.Errorf("argument %s: unknown type %q", name, typ)
}
