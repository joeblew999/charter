package pages

import (
	"context"

	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/intl"
)

// docURL is a docs page's address in the reader's language.
func docURL(ctx context.Context, d app.Doc) string { return intl.URL(ctx, d.Path()) }

// proseClass styles the HTML the Markdown became: Tailwind classes on the elements inside the article, in gsxui's
// tokens, since the HTML itself carries none.
const proseClass = "flex max-w-3xl flex-col gap-4 leading-7 " +
	"[&_h1]:text-3xl [&_h1]:font-semibold [&_h1]:tracking-tight " +
	"[&_h2]:mt-6 [&_h2]:scroll-mt-4 [&_h2]:border-b [&_h2]:pb-2 [&_h2]:text-xl [&_h2]:font-semibold " +
	"[&_h3]:mt-2 [&_h3]:scroll-mt-4 [&_h3]:text-lg [&_h3]:font-medium " +
	"[&_a]:underline [&_a]:underline-offset-4 " +
	"[&_ul]:list-disc [&_ul]:space-y-1 [&_ul]:ps-6 [&_ol]:list-decimal [&_ol]:space-y-1 [&_ol]:ps-6 " +
	"[&_code]:rounded [&_code]:bg-muted [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-sm " +
	"[&_pre]:overflow-x-auto [&_pre]:rounded-lg [&_pre]:border [&_pre]:bg-muted [&_pre]:p-4 [&_pre_code]:bg-transparent [&_pre_code]:p-0 " +
	"[&_table]:w-full [&_table]:text-sm [&_th]:border-b [&_th]:px-3 [&_th]:py-2 [&_th]:text-start [&_th]:font-medium " +
	"[&_td]:border-b [&_td]:px-3 [&_td]:py-2 [&_td]:align-top"

func tocClass(depth int) string {
	if depth > 2 {
		return "ps-3"
	}
	return ""
}

func docLinkClass(current bool) string {
	if current {
		return "rounded-md bg-muted px-3 py-1.5 text-sm font-medium"
	}
	return "rounded-md px-3 py-1.5 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
}
