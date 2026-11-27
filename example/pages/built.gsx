package pages

import (
	"strconv"

	"github.com/gsxhq/gsx"

	"github.com/joeblew999/charter/example/built"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/pages/screens"
	"github.com/joeblew999/charter/example/ui"
	"github.com/joeblew999/charter/intl"
)

// The pages that show what the app is built with (the group Built, package built). Every time and number on them
// is shown by Intl, in the reader's language and time zone (the since, date, number and duration filters; a
// message's plural). The names of what Cloudflare has (KV, R2) are names, and stay as they are.

// Page is the overview: each job and what does it, and what Cloudflare says of the reader's own request.
component (p BuiltIndex) Page(f built.Facts) {
	<Layout title={locales.BuiltTitle} description={locales.BuiltDescription}>
		<h1 class="text-2xl font-semibold">{ locales.BuiltHeading }</h1>
		<p class="text-muted-foreground">{ locales.BuiltIntro }</p>
		<ul id="built" class="flex flex-col gap-2">
			<BuiltLink
				href={AttachmentsPage{} |> url}
				title={locales.BuiltAttachmentsTitle}
				about={locales.BuiltAttachmentsAbout}
				uses={[]string{"R2", "D1"}}
			/>
			<BuiltLink
				href={ActivityPage{} |> url}
				title={locales.BuiltActivityTitle}
				about={locales.BuiltActivityAbout}
				uses={[]string{"Queues", "Cron Triggers", "D1", "Durable Objects"}}
			/>
			<BuiltLink
				href={NoticePage{} |> url}
				title={locales.BuiltNoticeTitle}
				about={locales.BuiltNoticeAbout}
				uses={[]string{"KV"}}
			/>
			<BuiltLink
				href={MailPage{} |> url}
				title={locales.BuiltMailTitle}
				about={locales.BuiltMailAbout}
				uses={[]string{"Email"}}
			/>
			<BuiltLink
				href={LimitsPage{} |> url}
				title={locales.BuiltLimitsTitle}
				about={locales.BuiltLimitsAbout}
				uses={[]string{"Rate Limiting", "Cache API"}}
			/>
			<BuiltLink
				href={OutsidePage{} |> url}
				title={locales.BuiltOutsideTitle}
				about={locales.BuiltOutsideAbout}
				uses={[]string{"fetch", "TCP sockets"}}
			/>
		</ul>
		<ui.Card>
			<ui.CardHeader>
				<ui.CardTitle>
					<h2>{ locales.BuiltFacts }</h2>
				</ui.CardTitle>
			</ui.CardHeader>
			<ui.CardContent>
				<dl id="facts" class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 text-sm">
					<dt class="text-muted-foreground">{ locales.BuiltWorker }</dt>
					<dd translate="no">{ f.Name }</dd>
					{ if f.Country != "" {
						<dt class="text-muted-foreground">{ locales.BuiltCountry }</dt>
						<dd>{ f.Country |> intl("DisplayNames", "of", "type", "region") }</dd>
					} }
					{ if f.City != "" {
						<dt class="text-muted-foreground">{ locales.BuiltCity }</dt>
						<dd translate="no">{ f.City }</dd>
					} }
					{ if f.Timezone != "" {
						<dt class="text-muted-foreground">{ locales.BuiltTimeZone }</dt>
						<dd>{ f.Timezone |> zone }</dd>
					} }
					{ if f.Colo != "" {
						<dt class="text-muted-foreground">{ locales.BuiltColo }</dt>
						<dd translate="no">{ f.Colo }</dd>
					} }
				</dl>
			</ui.CardContent>
		</ui.Card>
	</Layout>
}

// BuiltLink is one job on the overview: its page, what it is, and what of Cloudflare's does it.
component BuiltLink(href string, title intl.Msg, about intl.Msg, uses []string) {
	<li>
		<a href={href} class="flex flex-col gap-1 rounded-lg border p-3 hover:bg-muted">
			<span class="flex flex-wrap items-center gap-2">
				<span class="font-medium">{ title }</span>
				{ for _, name := range uses {
					<ui.Badge variant="secondary" translate="no">{ name }</ui.Badge>
				} }
			</span>
			<span class="text-sm text-muted-foreground">{ about }</span>
		</a>
	</li>
}

// BuiltPage is the frame of each job's page: the way back, what the page is, and what of Cloudflare's it uses.
component BuiltPage(title intl.Msg, about intl.Msg, uses []string, children gsx.Node) {
	<Layout title={title} description={about}>
		<ui.Button variant="link" class="self-start px-0" href={BuiltIndex{} |> url}>{ locales.BuiltBack }</ui.Button>
		<div class="flex flex-col gap-2">
			<h1 class="flex flex-wrap items-center gap-2 text-2xl font-semibold">
				{ title }
				{ for _, name := range uses {
					<ui.Badge variant="secondary" translate="no">{ name }</ui.Badge>
				} }
			</h1>
			<p class="text-muted-foreground">{ about }</p>
		</div>
		{ children }
	</Layout>
}

// SignInFirst is where something that needs someone signed in would be, for a reader who is not.
component SignInFirst() {
	<p class="text-sm text-muted-foreground">
		<ui.Button variant="link" class="px-0" href={screens.SignInPage{} |> url}>{ locales.BuiltSignIn }</ui.Button>
	</p>
}

component (p AttachmentsPage) Page(v AttachmentsView) {
	<BuiltPage title={locales.BuiltAttachmentsTitle} about={locales.BuiltAttachmentsAbout} uses={[]string{"R2", "D1"}}>
		{ if v.Caller.May(AttachFile{}.Needs()...) {
			<form
				id="attach"
				method="post"
				enctype="multipart/form-data"
				action={AttachFile{} |> url}
				class="flex flex-wrap items-center gap-2"
			>
				<ui.Input type="file" name="file" required class="flex-1" aria-label={locales.BuiltFile}/>
				<ui.NativeSelect name="note" aria-label={locales.BuiltNote}>
					<ui.NativeSelectOption value="0">{ locales.BuiltNoNote }</ui.NativeSelectOption>
					{ for _, note := range v.Notes {
						<ui.NativeSelectOption value={strconv.FormatInt(note.ID, 10)}>
							{ note.Body |> truncate(40) }
						</ui.NativeSelectOption>
					} }
				</ui.NativeSelect>
				<ui.Button type="submit">{ locales.BuiltAttach }</ui.Button>
			</form>
		} else {
			<SignInFirst/>
		} }
		<ul id="files" class="flex flex-col gap-2">
			{ for _, file := range v.Files {
				<p.File file={file} removable={built.MayDetach(v.Caller, file)}/>
			} }
		</ul>
		{ if len(v.Files) == 0 {
			<p class="text-sm text-muted-foreground">{ locales.BuiltAttachmentsEmpty }</p>
		} }
	</BuiltPage>
}

// File is one attached file: its name, which downloads it, how large it is, whose it is and when, each as Intl says
// it. Whoever may take it away has the button for that.
component (p AttachmentsPage) File(file built.Attachment, removable bool) {
	<li
		id={AttachmentsPage.File |> key(strconv.FormatInt(file.ID, 10))}
		class="flex flex-wrap items-center gap-2 rounded-md border px-3 py-2"
	>
		<a href={AttachedFile{} |> url(map[string]any{"file": file.ID})} class="flex-1 underline underline-offset-4" dir="auto" translate="no">{ file.Name }</a>
		<span class="text-xs text-muted-foreground">{ file.Size |> number("style", "unit", "unit", "byte", "notation", "compact", "unitDisplay", "narrow") }</span>
		<span class="text-xs text-muted-foreground" dir="auto" translate="no">{ file.Author }</span>
		<span class="text-xs text-muted-foreground">{ file.At() |> since }</span>
		{ if removable {
			<ui.Button
				variant="ghost"
				size="sm"
				type="button"
				data-remove
				hx-post={DetachFile{} |> url(map[string]any{"file": file.ID})}
				hx-target={AttachmentsPage.File |> keytarget(strconv.FormatInt(file.ID, 10))}
				hx-swap="delete"
			>
				{ locales.BuiltDetach }
			</ui.Button>
		} }
	</li>
}

// The activity: the list follows its feed from the newest row it shows, as the notes' list follows the notes.
// Rows are rows of the page, newest first: what it starts with, and what ActivitySince answers with.
component (p ActivityPage) Rows(rows []built.Activity) {
	{ for _, a := range rows {
		<p.Row a={a}/>
	} }
}

component (p ActivityPage) Page(v ActivityView) {
	<BuiltPage
		title={locales.BuiltActivityTitle}
		about={locales.BuiltActivityAbout}
		uses={[]string{"Queues", "Cron Triggers", "D1", "Durable Objects"}}
	>
		<ul
			id={ActivityPage.Row |> id}
			class="flex flex-col gap-2"
			hx-sse:connect={activitySignal(ctx, v.After)}
			hx-swap="afterbegin"
		>
			<p.Rows rows={v.Latest}/>
		</ul>
		{ if len(v.Latest) == 0 {
			<p class="text-sm text-muted-foreground">{ locales.BuiltActivityEmpty }</p>
		} }
	</BuiltPage>
}

// Row is one thing that happened: a note someone wrote, or a day's count, said in the reader's language (the count
// with its plural), and how long ago.
component (p ActivityPage) Row(a built.Activity) {
	<li data-kind={a.Kind} class="flex flex-wrap items-center gap-2 rounded-md border px-3 py-2 text-sm">
		<span class="flex-1" dir="auto">
			{ if a.Kind == built.Summary {
				{ locales.BuiltSummary(a.Notes) }
			} else {
				{ locales.BuiltWrote(a.Author) }
			} }
		</span>
		<span class="text-xs text-muted-foreground">{ a.At() |> since }</span>
	</li>
}

component (p NoticePage) Page(v NoticeView) {
	<BuiltPage title={locales.BuiltNoticeTitle} about={locales.BuiltNoticeAbout} uses={[]string{"KV"}}>
		<ui.Card>
			<ui.CardContent class="flex flex-col gap-2 pt-6">
				{ if v.Notice.Text == "" {
					<p id="notice-now" class="text-sm text-muted-foreground">{ locales.BuiltNoticeNone }</p>
				} else {
					<p id="notice-now" dir="auto">{ v.Notice.Text }</p>
					<p class="text-xs text-muted-foreground">{ v.Notice.Set |> since }</p>
				} }
				<p class="text-xs text-muted-foreground">{ locales.BuiltNoticeDelay }</p>
			</ui.CardContent>
		</ui.Card>
		{ if v.MaySet {
			<form id="set-notice" method="post" action={NoticeSet{} |> url} class="flex flex-wrap gap-2">
				<ui.Input
					name="notice"
					class="flex-1"
					value={v.Notice.Text}
					maxlength={strconv.Itoa(built.MaxNotice)}
					autocomplete="off"
					dir="auto"
					placeholder={locales.BuiltNoticeLine}
					aria-label={locales.BuiltNoticeLine}
				/>
				<ui.Button type="submit">{ locales.BuiltNoticeSet }</ui.Button>
			</form>
		} }
	</BuiltPage>
}

component (p MailPage) Page(v MailView) {
	<BuiltPage title={locales.BuiltMailTitle} about={locales.BuiltMailAbout} uses={[]string{"Email"}}>
		<ui.Card>
			<ui.CardContent class="flex flex-col gap-2 pt-6 text-sm">
				{ if v.Mail.Sent.IsZero() {
					<p id="sent" class="text-muted-foreground">{ locales.BuiltMailNoneSent }</p>
				} else {
					<p id="sent">{ locales.BuiltMailSent(v.Mail.SentTo, intl.Since(v.Mail.Sent)) }</p>
				} }
				{ if v.Mail.Received.IsZero() {
					<p id="received" class="text-muted-foreground">{ locales.BuiltMailNoneReceived }</p>
				} else {
					<p id="received">
						{ locales.BuiltMailReceived(v.Mail.From, intl.Since(v.Mail.Received)) }
						<span class="block text-muted-foreground" dir="auto" translate="no">{ v.Mail.Subject }</span>
					</p>
				} }
				{ if !v.Mail.MayNotSendBefore.IsZero() {
					<p class="text-xs text-muted-foreground">{ locales.BuiltMailWait(intl.Since(v.Mail.MayNotSendBefore)) }</p>
				} }
			</ui.CardContent>
		</ui.Card>
		{ if v.MaySend {
			<form method="post" action={MailSend{} |> url}>
				<ui.Button type="submit" id="send-mail">{ locales.BuiltMailSend }</ui.Button>
			</form>
		} else {
			<SignInFirst/>
		} }
	</BuiltPage>
}

component (p LimitsPage) Page(report built.Report, limit built.Limit) {
	<BuiltPage
		title={locales.BuiltLimitsTitle}
		about={locales.BuiltLimitsAbout}
		uses={[]string{"Rate Limiting", "Cache API"}}
	>
		<ui.Card>
			<ui.CardContent class="flex flex-col gap-3 pt-6">
				<p.Limit limit={limit}/>
			</ui.CardContent>
		</ui.Card>
		<ui.Card>
			<ui.CardContent class="flex flex-col gap-1 pt-6">
				<p id="report" class="text-lg font-medium">{ locales.BuiltReport(report.Notes, report.Authors) }</p>
				<p id="report-from" data-cached={strconv.FormatBool(report.FromCache)} class="text-sm text-muted-foreground">
					{ if report.FromCache {
						{ locales.BuiltReportCached(intl.Since(report.At)) }
					} else {
						{ locales.BuiltReportFresh }
					} }
				</p>
				<p class="text-xs text-muted-foreground">{ locales.BuiltTook(intl.Duration(report.Took, "")) }</p>
			</ui.CardContent>
		</ui.Card>
	</BuiltPage>
}

// Limit is the write limit and the button that uses one of it: a region of the page that changes by itself, to say
// whether the call was within the limit.
component (p LimitsPage) Limit(limit built.Limit) {
	<div id={LimitsPage.Limit |> id} class="flex flex-col gap-3">
		<p>{ locales.BuiltLimitIs(limit.Calls, limit.Seconds) }</p>
		<ui.Button
			type="button"
			variant="outline"
			class="self-start"
			hx-post={LimitUse{} |> url}
			hx-target={LimitsPage.Limit |> target}
			hx-swap="outerHTML"
		>
			{ locales.BuiltLimitUse }
		</ui.Button>
		{ if limit.Asked {
			<p id="limit-said" data-allowed={strconv.FormatBool(limit.Allowed)} class="text-sm">
				{ if limit.Allowed {
					{ locales.BuiltLimitAllowed }
				} else {
					{ locales.BuiltLimitRefused }
				} }
			</p>
		} }
	</div>
}

component (p OutsidePage) Page() {
	<BuiltPage
		title={locales.BuiltOutsideTitle}
		about={locales.BuiltOutsideAbout}
		uses={[]string{"fetch", "TCP sockets"}}
	>
		<div class="flex flex-wrap gap-2">
			<ui.Button
				type="button"
				id="fetch"
				hx-post={OutsideFetch{} |> url}
				hx-target={OutsidePage.Said |> target}
				hx-swap="outerHTML"
			>
				{ locales.BuiltOutsideFetch }
			</ui.Button>
			<ui.Button
				type="button"
				id="socket"
				variant="outline"
				hx-post={OutsideSocket{} |> url}
				hx-target={OutsidePage.Said |> target}
				hx-swap="outerHTML"
			>
				{ locales.BuiltOutsideSocket }
			</ui.Button>
		</div>
		<p.Said out={built.Outside{}}/>
	</BuiltPage>
}

// Said is what the other site answered and how long it took: empty until a button is pressed.
component (p OutsidePage) Said(out built.Outside) {
	<div id={OutsidePage.Said |> id} class="flex flex-col gap-1 text-sm">
		{ if out.Asked != "" {
			<p class="text-muted-foreground" translate="no">{ out.Asked }</p>
			<p id="outside-said" data-failed={strconv.FormatBool(out.Failed)} translate="no">{ out.Said }</p>
			{ if out.Failed {
				<p>{ locales.BuiltOutsideFailed }</p>
			} }
			<p class="text-xs text-muted-foreground">{ locales.BuiltTook(intl.Duration(out.Took, "")) }</p>
		} }
	</div>
}
