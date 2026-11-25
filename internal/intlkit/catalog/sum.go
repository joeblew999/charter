package catalog

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// intl.sum is, for every translated message, the source text it was translated from and the translation, as short
// hashes: one line per locale and key, sorted, so it reads as a diff. mise-intl gen writes it; it is committed, like
// go.sum, and never edited by hand.
//
//	de board.notes 3f2a9c1b7e04 9d81c0aa5f13
//
// It also records each file mise-intl add copied into the app (one of intl's screens) with the hash of what it
// wrote, so add can tell a file the app changed from one it may update:
//
//	file views/intl/preferences.gsx 5be0c2d9a7f1
type sum map[string]map[string]sumEntry // locale → key → entry

type sumEntry struct{ source, translation string }

const sumHeader = "# intl.sum: the source text each translation was made from (mise-intl gen writes it: commit it, don't edit it).\n"

// SumFile is intl.sum's path.
func (p *Project) SumFile() string { return filepath.Join(p.Config.CatalogDir(), "intl.sum") }

func (s sum) get(id, key string) (sumEntry, bool) {
	e, ok := s[id][key]
	return e, ok
}

func (s sum) set(id, key string, e sumEntry) {
	if s[id] == nil {
		s[id] = map[string]sumEntry{}
	}
	s[id][key] = e
}

func hash(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:6])
}

// FileHash is what intl.sum records of a file mise-intl add wrote.
func FileHash(b []byte) string { return hash(string(b)) }

// Copied is the hash intl.sum records for the file mise-intl add wrote at path (relative to the app's root), "" when
// it didn't write it.
func (p *Project) Copied(path string) string { return p.files[path] }

// SetCopied records that mise-intl add wrote path, hash its FileHash; WriteSum saves it.
func (p *Project) SetCopied(path, hash string) { p.files[path] = hash }

func readSum(path string) (sum, map[string]string, error) {
	s, files := sum{}, map[string]string{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, files, nil
	} else if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[0] == "file" {
			files[fields[1]] = fields[2]
			continue
		}
		if len(fields) < 4 {
			return nil, nil, fmt.Errorf("intl: %s:%d: want <locale> <key> <source hash> <translation hash>", path, n)
		}
		// A key with spaces is quoted: the hashes are the last two fields.
		key := strings.Join(fields[1:len(fields)-2], " ")
		if k, err := strconv.Unquote(key); err == nil {
			key = k
		}
		s.set(fields[0], key, sumEntry{source: fields[len(fields)-2], translation: fields[len(fields)-1]})
	}
	return s, files, sc.Err()
}

// sumBytes is intl.sum as it should be: the project's locales in intl.toml order, keys sorted.
func (p *Project) sumBytes() []byte {
	var b bytes.Buffer
	b.WriteString(sumHeader)
	for _, l := range p.Locales[1:] {
		keys := make([]string, 0, len(p.sum[l.ID]))
		for k := range p.sum[l.ID] {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			e := p.sum[l.ID][k]
			key := k
			if strings.ContainsAny(k, " \t\"") {
				key = strconv.Quote(k)
			}
			fmt.Fprintf(&b, "%s %s %s %s\n", l.ID, key, e.source, e.translation)
		}
	}
	for _, f := range slices.Sorted(maps.Keys(p.files)) {
		fmt.Fprintf(&b, "file %s %s\n", f, p.files[f])
	}
	return b.Bytes()
}

// WriteSum writes intl.sum.
func (p *Project) WriteSum() error { return writeFile(p.SumFile(), p.sumBytes()) }

// SumCurrent reports whether intl.sum on disk is what WriteSum would write.
func (p *Project) SumCurrent() bool {
	b, err := os.ReadFile(p.SumFile())
	return err == nil && bytes.Equal(b, p.sumBytes())
}

// writeFile writes b to path unless it already holds b, so tasks that track outputs see no change.
func writeFile(path string, b []byte) error {
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, b) {
		return nil
	}
	return os.WriteFile(path, b, 0o644)
}
