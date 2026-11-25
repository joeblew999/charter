package seo

import (
	"testing"

	"github.com/joeblew999/charter/internal/seokit/searchconsole"
)

func TestClassify(t *testing.T) {
	const u = "https://app.example.com/en-in/about"
	const link = "https://search.google.com/search-console/inspect?resource_id=sc-domain:example.com&id=abc123"
	for _, tc := range []struct {
		name          string
		in            searchconsole.Inspection
		show, problem bool
	}{
		{"indexed", searchconsole.Inspection{Verdict: "PASS", CoverageState: "Submitted and indexed"}, false, false},
		{"waiting", searchconsole.Inspection{Verdict: "NEUTRAL", CoverageState: "Discovered - currently not indexed", Link: link}, true, false},
		{"unknown", searchconsole.Inspection{Verdict: "NEUTRAL", CoverageState: "URL is unknown to Google", Link: link}, true, false},
		{"duplicate", searchconsole.Inspection{Verdict: "NEUTRAL", CoverageState: "Duplicate, Google chose different canonical than user",
			GoogleCanonical: "https://app.example.com/about", Link: link}, true, false},
		{"404", searchconsole.Inspection{Verdict: "FAIL", CoverageState: "Not found (404)", Link: link}, true, true},
		{"noindex", searchconsole.Inspection{Verdict: "NEUTRAL", CoverageState: "Excluded by ‘noindex’ tag", Link: link}, true, true},
		{"crawled not indexed", searchconsole.Inspection{Verdict: "NEUTRAL", CoverageState: "Crawled - currently not indexed", Link: link}, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, show := classify(u, tc.in)
			if show != tc.show || f.Problem != tc.problem {
				t.Fatalf("classify = show %v problem %v, want show %v problem %v (%+v)", show, f.Problem, tc.show, tc.problem, f)
			}
			if show && f.Link != tc.in.Link {
				t.Errorf("link = %q, want Google's %q", f.Link, tc.in.Link)
			}
		})
	}
}
