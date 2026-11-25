package catalog

import "encoding/json"

// Tree is src parsed and encoded as Generate writes it for js/intl.mjs, with its argument names in index order.
func Tree(src string) (json.RawMessage, []string, error) {
	p, err := parse(src)
	if err != nil {
		return nil, nil, err
	}
	m := &Message{parsed: p, index: map[string]int8{}}
	for i, name := range p.order {
		m.index[name] = int8(i)
	}
	b, err := json.Marshal(jsMessage(p.parts, m))
	return b, p.order, err
}

// ParseError is the error parsing src as ICU MessageFormat, or nil.
func ParseError(src string) error {
	_, err := parse(src)
	return err
}
