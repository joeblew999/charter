package screens

import (
	"net/http"

	"github.com/jackielii/structpages"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/ui"
	"github.com/joeblew999/charter/example/ui/icon"
	"github.com/joeblew999/charter/intl"
)

// Preferences is the time zone and clock dialog's two routes: its form, which loads when the dialog opens, and
// where the form is sent. Mount it in the tree:
//
//	preferences screens.Preferences `route:"/preferences Preferences"`
//
// From charter add screen:preferences: the app's own now, to change as it likes.
type Preferences struct {
	fields PreferencesFields `route:"GET /{$} PreferencesFields"`
	save   PreferencesSave   `route:"POST /{$} PreferencesSave"`
}

// PreferencesFields is the dialog's form, a fragment: it is swapped into the dialog and is never a page.
type PreferencesFields struct{}

func (PreferencesFields) Props(r *http.Request, site intl.Site) error {
	return structpages.RenderComponent(PreferencesForm(intl.ReturnTo(r), site.ConnectionZone(r)))
}

// PreferencesSave keeps the choice (intl.Site's Preferences, which answers with a redirect back to the page).
type PreferencesSave struct{}

func (PreferencesSave) ServeHTTP(w http.ResponseWriter, r *http.Request, site intl.Site) {
	site.Preferences(w, r)
}

// PreferencesDialog is a header button and the time zone and clock dialog it opens. The dialog's form loads when it
// opens (PreferencesFields), swapped in with innerHTML: inside a boosted nav, an inherited outerMorph would replace
// the target itself.
component PreferencesDialog() {
	<ui.Dialog>
		<ui.Button
			variant="ghost"
			size="icon-sm"
			data-gsxui-slot-dialog-trigger
			aria-haspopup="dialog"
			aria-expanded="false"
			aria-label={locales.ScreensPreferences}
			title={locales.ScreensPreferences}
			hx-get={PreferencesFields{} |> url}
			hx-target="#preferences"
			hx-swap="innerHTML"
		>
			<icon.Clock/>
		</ui.Button>
		<ui.DialogContent hideCloseButton={true}>
			<ui.DialogHeader>
				<ui.DialogTitle>{ locales.ScreensPreferences }</ui.DialogTitle>
				<ui.DialogDescription>{ locales.ScreensPreferencesDescription }</ui.DialogDescription>
			</ui.DialogHeader>
			<div id="preferences" class="text-sm text-muted-foreground">{ locales.ScreensLoading }</div>
		</ui.DialogContent>
	</ui.Dialog>
}

// PreferencesForm is the form: a plain POST (PreferencesSave); saving redirects back to the page, which renders
// again with the new <html data-tz data-hc>. The zone list is the browser's own (Intl.supportedValuesOf, filled in by
// intl's browser module), each named in the page's language; without JavaScript it offers automatic and the current
// choice. The clock options are labelled by an example time in each clock.
component PreferencesForm(returnTo string, connectionZone string) {
	<form method="post" action={PreferencesSave{} |> url} class="flex flex-col gap-4">
		<input type="hidden" name="return" value={returnTo}/>
		<ui.FieldGroup>
			<ui.Field>
				<ui.FieldLabel for="pref-tz">{ locales.ScreensTimeZone }</ui.FieldLabel>
				<ui.NativeSelect id="pref-tz" name="tz" class="w-full" data-intl-zones>
					<ui.NativeSelectOption
						value=""
						selected={intl.ChosenTimeZone(ctx) == ""}
						data-intl="browser-zone"
						data-template={locales.ScreensTimeZoneAuto(intl.Slot)}
					>
						{ locales.ScreensTimeZoneAuto(connectionZone) }
					</ui.NativeSelectOption>
					{ if intl.ChosenTimeZone(ctx) != "" {
						<ui.NativeSelectOption
							value={intl.TimeZone(ctx)}
							selected={true}
							{ intl.Attrs(intl.Zone(intl.TimeZone(ctx)))... }
						>
							{ intl.TimeZone(ctx) |> zone |> plain }
						</ui.NativeSelectOption>
					} }
				</ui.NativeSelect>
			</ui.Field>
			<ui.Field>
				<ui.FieldLabel for="pref-hc">{ locales.ScreensHourCycle }</ui.FieldLabel>
				<ui.NativeSelect id="pref-hc" name="hc" class="w-full">
					<ui.NativeSelectOption
						value=""
						selected={intl.HourCycle(ctx) == ""}
						{ intl.Attrs(intl.Clock(""))... }
						data-template={locales.ScreensHourCycleAuto(intl.Slot)}
					>
						{ locales.ScreensHourCycleAuto(intl.Clock("")) }
					</ui.NativeSelectOption>
					<ui.NativeSelectOption
						value="h12"
						selected={intl.HourCycle(ctx) == "h12"}
						{ intl.Attrs(intl.Clock("h12"))... }
					>
						{ intl.Clock("h12") |> plain }
					</ui.NativeSelectOption>
					<ui.NativeSelectOption
						value="h23"
						selected={intl.HourCycle(ctx) == "h23"}
						{ intl.Attrs(intl.Clock("h23"))... }
					>
						{ intl.Clock("h23") |> plain }
					</ui.NativeSelectOption>
				</ui.NativeSelect>
			</ui.Field>
		</ui.FieldGroup>
		<ui.DialogFooter showCloseButton={false}>
			<ui.Button variant="outline" type="button" data-gsxui-dialog-close data-gsxui-slot-dialog-footer-close>
				{ locales.ScreensClose }
			</ui.Button>
			<ui.Button type="submit">{ locales.ScreensSave }</ui.Button>
		</ui.DialogFooter>
	</form>
}
