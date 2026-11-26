package live

import "strings"

// Data is text as data lines: each of its lines (SSE takes CR, LF and CRLF alike as line breaks)
// becomes "data: " + prefix + the line. Data("", html) is htmx's; Data("elements ", html) Datastar's.
func Data(prefix, text string) string {
	start := "data: " + prefix
	return start + strings.NewReplacer("\r\n", "\n"+start, "\r", "\n"+start, "\n", "\n"+start).Replace(text)
}
