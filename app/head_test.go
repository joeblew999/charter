package app

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

// Under charter dev the stylesheet comes by script, after the page's markup: the page is hidden until it has come,
// so it is never seen bare. What is built for a deploy links its stylesheet in the head, and has none of it.
func TestThePageIsHiddenUntilStyledUnderDevOnly(t *testing.T) {
	for dev, want := range map[bool]bool{true: true, false: false} {
		var out bytes.Buffer
		if err := styled(dev).Render(context.Background(), &out); err != nil {
			t.Fatal(err)
		}
		has := strings.Contains(out.String(), "html[data-loading] body") && strings.Contains(out.String(), `dataset.loading = "true"`) && strings.Contains(out.String(), "DOMContentLoaded")
		if has != want {
			t.Errorf("dev %v: hidden until styled is %v, want %v:\n%s", dev, has, want, out.String())
		}
	}
}
