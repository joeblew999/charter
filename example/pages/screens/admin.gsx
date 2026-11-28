package screens

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gsxhq/gsx"
	"github.com/jackielii/structpages"
	"github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/cloud"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/ui/icon"
	ui "github.com/joeblew999/charter/example/ui/styles"
	"github.com/joeblew999/charter/intl"
)

// Admin is what an admin sees of the system: the app at a glance, what the Worker is bound to, and what its
// databases hold. Each page asks only for what it shows, and nothing here changes anything but an invitation
// that is cancelled. Mount it in the tree, beside the sign-in screen it needs:
//
//	admin screens.Admin `route:"/admin Admin"`
//
// and put <screens.AdminLink/> in the header: it shows for whoever may come here, and for no one else.
type Admin struct {
	overview AdminOverview `route:"GET /{$} Admin"`
	cancel   InviteCancel  `route:"POST /invitations/{invitation}/cancel InviteCancel"`
	system   AdminSystem   `route:"GET /system AdminSystem"`
	probe    AdminProbe    `route:"POST /system/{binding}/probe AdminProbe"`
	database AdminDatabase `route:"GET /database AdminDatabase"`
}

// Needs is what every route here needs: the scope of whoever runs the app. Someone signed in without it gets a 403.
func (Admin) Needs() []api.Scope { return []api.Scope{api.Admin} }

// AdminLink is the way to the admin pages, for a header: there for whoever may open them.
component AdminLink(attrs gsx.Attrs) {
	{ if api.Current(ctx).May(Admin{}.Needs()...) {
		<ui.Button variant="ghost" size="sm" href={AdminOverview{} |> url} id="admin-link" { attrs... }>
			<icon.Settings/>
			{ locales.ScreensAdmin }
		</ui.Button>
	} }
}

// adminPage is one of the admin's pages: the way between them, the one shown marked, and the page.
component adminPage(title intl.Msg, about intl.Msg, here string, children gsx.Node) {
	<Layout
		mode={LayoutPage}
		title={title |> text}
		description={about |> text}
		heading={title |> text}
		about={about |> text}
		tabs={ <AdminTabs here={here}/> }
	>
		{ children }
	</Layout>
}

// AdminTabs is the way between the admin's pages, with the one being shown marked. The people's page, which is the
// sign-in screen's, has it too.
component AdminTabs(here string) {
	<Tabs
		id="admin-nav"
		aria-label={locales.ScreensAdmin}
		tabs={
			[]Tab{
				{Href: adminURL(ctx, AdminOverview{}), Name: intl.Text(ctx, locales.ScreensAdminOverview), Here: here == "overview"},
				{Href: adminURL(ctx, PeoplePage{}), Name: intl.Text(ctx, locales.ScreensPeople), Here: here == "people"},
				{Href: adminURL(ctx, AdminSystem{}), Name: intl.Text(ctx, locales.ScreensAdminSystem), Here: here == "system"},
				{Href: adminURL(ctx, AdminDatabase{}), Name: intl.Text(ctx, locales.ScreensAdminDatabase), Here: here == "database"},
			}
		}
	/>
}

// adminURL is a page's address, from the tree.
func adminURL(ctx context.Context, page any) string {
	to, _ := structpages.URLFor(ctx, page)
	return to
}

func init() { PeopleTabs = func() gsx.Node { return AdminTabs("people") } }

// AdminOverview is the app at a glance: what it is built from, who there is, the invitations that wait, and the
// app's own facts.
type AdminOverview struct{}

func (AdminOverview) Noindex() bool { return true }

// AdminOverviewView is what it shows.
type AdminOverviewView struct {
	api.Overview
	Facts []AdminFact
}

// AdminFact is one of the app's facts (app.App's Facts), with its value as it is now: a number, a time, or text.
type AdminFact struct {
	Label  intl.Msg
	Number *int64
	Time   *time.Time
	Text   string
}

func (AdminOverview) Props(r *http.Request, auth *api.Auth) (AdminOverviewView, error) {
	overview, err := auth.Overview(r)
	if err != nil {
		return AdminOverviewView{}, err
	}
	v := AdminOverviewView{Overview: overview}
	for _, fact := range app.Facts(r.Context()) {
		row, err := adminFact(r.Context(), fact)
		if err != nil {
			return v, err
		}
		v.Facts = append(v.Facts, row)
	}
	return v, nil
}

// adminFact asks a fact for its value.
func adminFact(ctx context.Context, fact app.Fact) (AdminFact, error) {
	row := AdminFact{Label: fact.Label}
	value, err := fact.Value(ctx)
	if err != nil {
		return row, err
	}
	switch v := value.(type) {
	case int:
		n := int64(v)
		row.Number = &n
	case int64:
		row.Number = &v
	case time.Time:
		row.Time = &v
	default:
		row.Text = fmt.Sprint(v)
	}
	return row, nil
}

// InviteCancel takes an invitation away before anyone takes it up, and answers with nothing: its row goes.
type InviteCancel struct{}

func (InviteCancel) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) error {
	if err := auth.CancelInvitation(r, r.PathValue("invitation")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusOK)
	return nil
}

component (p AdminOverview) Page(v AdminOverviewView) {
	<adminPage title={locales.ScreensAdminOverview} about={locales.ScreensAdminOverviewIntro} here="overview">
		<Panel title={v.App}>
			<Rows id="admin-build">
				<Row>
					<span class="flex-1" translate="no">charter</span>
					<code translate="no">{ v.Charter }</code>
				</Row>
				<Row>
					<span class="flex-1" translate="no">Go</span>
					<code translate="no">{ v.Go }</code>
				</Row>
				{ if v.Revision != "" {
					<Row>
						<span class="flex-1">{ locales.ScreensAdminRevision }</span>
						{ if v.Modified {
							<ui.Badge variant="secondary">{ locales.ScreensAdminModified }</ui.Badge>
						} }
						<code translate="no">{ v.Revision[:min(len(v.Revision), 12)] }</code>
					</Row>
				} }
				{ if !v.Committed.IsZero() {
					<Row>
						<span class="flex-1">{ locales.ScreensAdminCommitted }</span>
						<span>{ v.Committed |> date("dateStyle", "medium", "timeStyle", "short") }</span>
					</Row>
				} }
			</Rows>
		</Panel>
		<Panel title={locales.ScreensPeople |> text}>
			<Rows id="admin-people">
				{ for _, role := range v.People {
					<Row>
						<span class="flex-1" translate="no">{ role.Role }</span>
						<span data-role={role.Role}>{ role.Count |> number }</span>
					</Row>
				} }
			</Rows>
		</Panel>
		<Panel title={locales.ScreensAdminInvitations |> text} about={locales.ScreensAdminInvitationsIntro |> text}>
			{ if len(v.Invitations) == 0 {
				<None id="admin-invitations">{ locales.ScreensAdminNone }</None>
			} else {
				<Rows id="admin-invitations">
					{ for _, invitation := range v.Invitations {
						<p.Invitation invitation={invitation}/>
					} }
				</Rows>
			} }
		</Panel>
		{ if len(v.Facts) > 0 {
			<Panel title={locales.ScreensAdminFacts |> text}>
				<Rows id="admin-facts">
					{ for _, fact := range v.Facts {
						<Row>
							<span class="flex-1">{ fact.Label }</span>
							{ if fact.Number != nil {
								<span>{ *fact.Number |> number }</span>
							} else if fact.Time != nil {
								<span>{ *fact.Time |> date("dateStyle", "medium", "timeStyle", "short") }</span>
							} else {
								<span dir="auto">{ fact.Text }</span>
							} }
						</Row>
					} }
				</Rows>
			</Panel>
		} }
	</adminPage>
}

// Invitation is one invitation that waits: what it is to, until when, and the button that cancels it.
component (p AdminOverview) Invitation(invitation api.Invitation) {
	<Row id={AdminOverview.Invitation |> key(invitation.ID)}>
		<ui.Badge translate="no">{ invitation.Role }</ui.Badge>
		<span class="flex-1 text-muted-foreground">{ invitation.Expires |> date("dateStyle", "medium", "timeStyle", "short") }</span>
		<ui.Button
			type="button"
			variant="ghost"
			size="sm"
			hx-post={InviteCancel{} |> url(map[string]any{"invitation": invitation.ID})}
			hx-target={AdminOverview.Invitation |> keytarget(invitation.ID)}
			hx-swap="delete"
		>
			{ locales.ScreensAdminCancel }
		</ui.Button>
	</Row>
}

// AdminSystem is what the Worker is bound to, as it runs: its bindings, each with a button that asks it one small
// thing, and its variables and secrets by name.
type AdminSystem struct{}

func (AdminSystem) Noindex() bool { return true }

func (AdminSystem) Props(r *http.Request, auth *api.Auth) (api.System, error) { return auth.System(r) }

// AdminProbe asks one binding one small thing and answers with its row again, which says how it went.
type AdminProbe struct{}

func (AdminProbe) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) error {
	name := r.PathValue("binding")
	probed, err := auth.Probe(r, name)
	if err != nil {
		return err
	}
	return structpages.RenderComponent(AdminSystem{}.Binding(cloud.Binding{
		Name: name,
		Kind: r.PostFormValue("kind"),
	}, &probed))
}

component (p AdminSystem) Page(v api.System) {
	<adminPage title={locales.ScreensAdminSystem} about={locales.ScreensAdminSystemIntro} here="system">
		{ if v.Memory != nil {
			<Panel title={locales.ScreensAdminMemory |> text} about={locales.ScreensAdminMemoryIntro |> text}>
				<Rows id="admin-memory">
					<Row>
						<span class="flex-1">{ locales.ScreensAdminRuntimes }</span>
						<span>{ v.Memory.Runtimes |> number }</span>
					</Row>
					<Row>
						<span class="flex-1">{ locales.ScreensAdminHeld }</span>
						<span>{ (v.Memory.Held / 1048576) |> number("style", "unit", "unit", "megabyte") }</span>
					</Row>
					<Row>
						<span class="flex-1">{ locales.ScreensAdminBudget }</span>
						<span>{ (v.Memory.Budget / 1048576) |> number("style", "unit", "unit", "megabyte") }</span>
					</Row>
				</Rows>
			</Panel>
		} }
		<Panel title={locales.ScreensAdminBindings |> text}>
			<Rows id="admin-bindings">
				{ for _, binding := range v.Bindings {
					<p.Binding binding={binding} probed={nil}/>
				} }
			</Rows>
		</Panel>
		<Panel title={locales.ScreensAdminSettings |> text} about={locales.ScreensAdminSettingsIntro |> text}>
			<Rows id="admin-settings">
				{ for _, setting := range v.Settings {
					<Row>
						<code class="flex-1" translate="no">{ setting.Name }</code>
						{ if setting.Set {
							<ui.Badge variant="secondary">{ locales.ScreensAdminSet }</ui.Badge>
						} else {
							<ui.Badge variant="destructive">{ locales.ScreensAdminUnset }</ui.Badge>
						} }
					</Row>
				} }
			</Rows>
		</Panel>
	</adminPage>
}

// Binding is one binding: its name, its kind, and once it was asked, how it answered.
component (p AdminSystem) Binding(binding cloud.Binding, probed *api.Probed) {
	<Row id={AdminSystem.Binding |> key(binding.Name)}>
		<code translate="no">{ binding.Name }</code>
		<span class="flex-1 text-muted-foreground" translate="no">{ binding.Kind }</span>
		{ if probed != nil && probed.Asked && probed.OK {
			<ui.Badge variant="secondary" data-probed="ok">
				{ locales.ScreensAdminAnswers } · { probed.Took.Milliseconds() |> number("style", "unit", "unit", "millisecond") }
			</ui.Badge>
		} else if probed != nil && probed.Asked {
			<ui.Badge variant="destructive" data-probed="failed" title={probed.Error}>
				{ locales.ScreensAdminNoAnswer }
			</ui.Badge>
		} else if probed != nil {
			<ui.Badge variant="outline" data-probed="bound">{ locales.ScreensAdminBound }</ui.Badge>
		} }
		// Only what has something harmless to be asked: a queue takes only messages.
		{ if cloud.Probes(binding.Kind) {
			<form
				hx-post={AdminProbe{} |> url(map[string]any{"binding": binding.Name})}
				hx-target={AdminSystem.Binding |> keytarget(binding.Name)}
				hx-swap="outerHTML"
			>
				<input type="hidden" name="kind" value={binding.Kind}/>
				<ui.Button type="submit" variant="outline" size="sm">{ locales.ScreensAdminProbe }</ui.Button>
			</form>
		} }
	</Row>
}

// AdminDatabase is what the Worker's databases hold: each table with how many rows it has, and the migrations
// that ran.
type AdminDatabase struct{}

func (AdminDatabase) Noindex() bool { return true }

func (AdminDatabase) Props(r *http.Request, auth *api.Auth) ([]api.Database, error) {
	return auth.Databases(r)
}

component (p AdminDatabase) Page(databases []api.Database) {
	<adminPage title={locales.ScreensAdminDatabase} about={locales.ScreensAdminDatabaseIntro} here="database">
		{ for _, db := range databases {
			<Panel title={db.Binding} about={locales.ScreensAdminTables |> text}>
				<Rows data-tables={db.Binding}>
					{ for _, table := range db.Tables {
						<Row>
							<code class="flex-1" translate="no">{ table.Name }</code>
							<span data-table={table.Name}>{ table.Rows |> number }</span>
						</Row>
					} }
				</Rows>
			</Panel>
			<Panel title={locales.ScreensAdminMigrations |> text}>
				{ if len(db.Migrations) == 0 {
					<None>{ locales.ScreensAdminNone }</None>
				} else {
					<Rows data-migrations={db.Binding}>
						{ for _, migration := range db.Migrations {
							<Row>
								<code class="flex-1" translate="no">{ migration.Name }</code>
								<span class="text-muted-foreground">{ migration.Applied |> date("dateStyle", "medium", "timeStyle", "short") }</span>
							</Row>
						} }
					</Rows>
				} }
			</Panel>
		} }
	</adminPage>
}
