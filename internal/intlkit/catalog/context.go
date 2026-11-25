package catalog

import (
	"os"
	"strings"
)

// contexts are the comments translators read in the source catalog: for each key, the comment block directly above
// its table header and the one directly above the key (no blank line between).
//
//	# The shared board page.
//	[board]
//	# A count under the board; n is how many notes it shows.
//	notes = "{n, plural, one {# note} other {# notes}}"
func contexts(path string) (map[string]string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	var table, tableNote string
	var block []string
	multi := false
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if multi {
			multi = !strings.Contains(t, `"""`) && !strings.Contains(t, `'''`)
			continue
		}
		switch {
		case t == "":
			block = nil
		case t[0] == '#':
			block = append(block, strings.TrimSpace(strings.TrimPrefix(t, "#")))
		case t[0] == '[':
			table = tableName(t)
			tableNote, block = strings.Join(block, " "), nil
		default:
			name, value, ok := strings.Cut(t, "=")
			if !ok {
				block = nil
				continue
			}
			key := strings.Trim(strings.TrimSpace(name), `"`)
			if table != "" {
				key = table + "." + key
			}
			note := strings.TrimSpace(tableNote + " " + strings.Join(block, " "))
			if note != "" {
				out[key] = note
			}
			block = nil
			v := strings.TrimSpace(value)
			for _, q := range []string{`"""`, `'''`} {
				if strings.HasPrefix(v, q) && !strings.Contains(v[3:], q) {
					multi = true
				}
			}
		}
	}
	return out, nil
}
