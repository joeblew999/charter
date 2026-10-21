package searchconsole

import "unicode"

// Google cuts search result titles and descriptions by pixel width. These character budgets count a Latin letter as 1
// and a CJK or full-width character as 2 (they render about twice as wide), so one limit works for every locale.
const (
	MaxTitleWidth       = 60
	MaxDescriptionWidth = 160
)

// SnippetWidth is s's width in those units.
func SnippetWidth(s string) int {
	n := 0
	for _, r := range s {
		n++
		if wide(r) {
			n++
		}
	}
	return n
}

func wide(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) ||
		r >= 0x3000 && r <= 0x30FF || // CJK punctuation (、。「」) and the kana blocks, incl. ー which is script Common
		r >= 0x31F0 && r <= 0x31FF || // katakana phonetic extensions
		r >= 0xFF00 && r <= 0xFF60 || r >= 0xFFE0 && r <= 0xFFE6 // full-width forms (，：)
}
