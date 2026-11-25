package seo

import "strings"

// LLMsSection is one "## " section of an llms.txt: links to read, with a note on each.
type LLMsSection struct {
	Name  string
	Links []LLMsLink
}

// LLMsLink is a link in an LLMsSection. URL is absolute; Note may be empty.
type LLMsLink struct {
	Title, URL, Note string
}

// LLMsTxt writes an llms.txt as https://llmstxt.org lays it out: the site's name as its title, a one-line summary
// as a quote, details (Markdown, may be empty), then the sections.
func LLMsTxt(title, summary, details string, sections ...LLMsSection) string {
	var b strings.Builder
	b.WriteString("# " + title + "\n\n> " + summary + "\n")
	if details != "" {
		b.WriteString("\n" + strings.TrimSpace(details) + "\n")
	}
	for _, s := range sections {
		b.WriteString("\n## " + s.Name + "\n\n")
		for _, l := range s.Links {
			b.WriteString("- [" + l.Title + "](" + l.URL + ")")
			if l.Note != "" {
				b.WriteString(": " + l.Note)
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}
