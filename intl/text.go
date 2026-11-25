package intl

import (
	"context"
	"strconv"
	"strings"
)

// Segments is text split the request's language's way (Intl.Segmenter): granularity "grapheme" (what a reader sees
// as one character: "👨‍👩‍👧" is one), "word" (word-like segments only, Japanese and Thai included) or "sentence".
// len(Segments(…)) counts them; the first grapheme of each word makes initials.
func Segments(ctx context.Context, text, granularity string) []string {
	if text == "" {
		return nil
	}
	s, ok := call("segments", Current(ctx).id, text, granularity)
	if !ok || s == "" {
		return nil
	}
	return strings.Split(s, "\u001f")
}

// Order is the order Intl.Collator sorts items in, in the request's language, with opts (a JSON object of its
// options, "" for none): the indexes of items, first to last. Rows sort by a key through it:
//
//	for _, i := range intl.Order(ctx, names, "") { rows[i]… }
func Order(ctx context.Context, items []string, opts string) []int {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = JSON(s)
	}
	s, ok := call("order", Current(ctx).id, "["+strings.Join(q, ",")+"]", opts)
	out := make([]int, 0, len(items))
	if ok && s != "" {
		for _, f := range strings.Split(s, ",") {
			if i, err := strconv.Atoi(f); err == nil && i >= 0 && i < len(items) {
				out = append(out, i)
			}
		}
	}
	if len(out) != len(items) { // no Intl: as they are
		out = out[:0]
		for i := range items {
			out = append(out, i)
		}
	}
	return out
}
