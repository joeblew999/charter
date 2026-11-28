package screens

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gsxhq/gsx"
	"github.com/jackielii/structpages"
	"github.com/joeblew999/charter/api"
	"github.com/joeblew999/charter/app"
	"github.com/joeblew999/charter/example/locales"
	"github.com/joeblew999/charter/example/ui/icon"
	ui "github.com/joeblew999/charter/example/ui/styles"
	"github.com/joeblew999/charter/intl"
)

// SignIn is how the app's own people sign in: with a passkey, and no password (api.People). Its routes are the
// page, the four steps of the two ceremonies (api.Auth runs them), and signing out. Mount it in the tree, and
// name its page as where someone who must sign in is sent:
//
//	signIn  screens.SignIn  `route:"/sign-in SignIn"`
//	account screens.Account `route:"/account Account"`
//
//	(&app.App{Auth: auth, SignIn: screens.SignInPage{}, Args: []any{auth}, RequestArgs: []any{auth.Caller}, …})
//
// From charter add screen:sign-in: the app's own now, to change as it likes.
type SignIn struct {
	page     SignInPage   `route:"GET /{$} SignIn"`
	upBegin  SignUpBegin  `route:"POST /up/begin SignUpBegin"`
	upFinish SignUpFinish `route:"POST /up/finish SignUpFinish"`
	inBegin  SignInBegin  `route:"POST /begin SignInBegin"`
	inFinish SignInFinish `route:"POST /finish SignInFinish"`
	out      SignOut      `route:"POST /out SignOut"`
	accept   InviteAccept `route:"POST /accept InviteAccept"`
}

// SignInPage is the page: sign in, or make an account.
type SignInPage struct{}

// SignInView is what the page shows: who is signed in already, and where to go once someone is.
type SignInView struct {
	Caller api.Caller
	Return string
	// Invite is an invitation's code, from its link: whoever signs up here, or signs in and accepts, is what it
	// says (InviteRole). InviteGone: the link was used already, or is too old.
	Invite     string
	InviteRole string
	InviteGone bool
}

// Noindex keeps the page out of the sitemap and of search engines: it is a door, not something to find.
func (SignInPage) Noindex() bool { return true }

// FullLoad says a link to this page is a page load: its script runs as the document loads.
func (SignInPage) FullLoad() bool { return true }

func (SignInPage) Props(r *http.Request, caller api.Caller, auth *api.Auth) (SignInView, error) {
	v := SignInView{Caller: caller, Return: intl.SafeReturn(r.URL.Query().Get("return"))}
	if code := r.URL.Query().Get("invite"); code != "" {
		if role, ok := auth.Invited(r, code); ok {
			v.Invite, v.InviteRole = code, role
		} else {
			v.InviteGone = true
		}
	}
	return v, nil
}

// The ceremonies' steps: what the browser asks the device, and what the device answered. They speak JSON to the
// page's script, so each is a route that writes its own answer.
type (
	SignUpBegin  struct{}
	SignUpFinish struct{}
	SignInBegin  struct{}
	SignInFinish struct{}
	SignOut      struct{}
)

func (SignUpBegin) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) error {
	return auth.SignUpBegin(w, r)
}

func (SignUpFinish) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) error {
	return auth.SignUpFinish(w, r)
}

func (SignInBegin) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) error {
	return auth.SignInBegin(w, r)
}

func (SignInFinish) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) error {
	return auth.SignInFinish(w, r)
}

// InviteAccept gives whoever is signed in what an invitation says, and goes to their account, which shows it.
type InviteAccept struct{}

func (InviteAccept) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) error {
	if err := auth.Accept(r, r.PostFormValue("invite")); err != nil {
		return err
	}
	to, err := structpages.URLFor(r.Context(), AccountPage{})
	if err != nil {
		return err
	}
	return app.Redirect{To: to}
}

// SignOut ends the session and goes to the front page: a plain form's target.
func (SignOut) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) {
	if err := auth.SignOut(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, intl.URL(r.Context(), "/"), http.StatusSeeOther)
}

component (p SignInPage) Page(v SignInView) {
	// Here by an invitation's link: the page says so first, and what it is to.
	<Layout
		mode={LayoutPage}
		title={locales.ScreensSignIn |> text}
		description={locales.ScreensSignUpIntro |> text}
		heading={signInHeading(ctx, v)}
		about={signInAbout(ctx, v)}
	>
		<Panel hx-boost:inherited="false">
			{ if v.InviteGone {
				<p id="invite-gone" role="alert" class="text-sm text-destructive">{ locales.ScreensInviteGone }</p>
			} }
			{ if v.Caller.SignedIn() {
				<p id="signed-in">{ locales.ScreensSignedInAs(v.Caller.Name()) }</p>
				// Here by an invitation's link, with an account already: it is theirs to accept.
				{ if v.Invite != "" {
					<form method="post" action={InviteAccept{} |> url} id="accept">
						<input type="hidden" name="invite" value={v.Invite}/>
						<ui.Button type="submit">{ locales.ScreensInviteAccept }</ui.Button>
					</form>
				} }
				<div class="flex flex-wrap gap-2">
					<ui.Button variant="outline" size="sm" href={AccountPage{} |> url} id="account">
						{ locales.ScreensAccount }
					</ui.Button>
					<SignOutForm/>
				</div>
			} else {
				<div
					id="sign-in"
					class="flex flex-col gap-4"
					data-return={v.Return}
					data-invite={v.Invite}
					data-accept={InviteAccept{} |> url}
					data-account={AccountPage{} |> url}
					data-in-begin={SignInBegin{} |> url}
					data-in-finish={SignInFinish{} |> url}
					data-up-begin={SignUpBegin{} |> url}
					data-up-finish={SignUpFinish{} |> url}
					data-failed={locales.ScreensSignInFailed |> text}
				>
					// By an invitation, the account to make comes first: signing in is for whoever has one.
					{ if v.Invite != "" {
						<signUpForm variant="default"/>
						<ui.Separator/>
						<p class="text-sm text-muted-foreground">{ locales.ScreensHaveAccount }</p>
						<passkeyButton variant="outline"/>
					} else {
						<passkeyButton variant="default"/>
						<ui.Separator/>
						<p class="text-sm text-muted-foreground">{ locales.ScreensSignUpIntro }</p>
						<signUpForm variant="outline"/>
					} }
					<p id="sign-in-error" role="alert" class="text-sm text-destructive" hidden></p>
				</div>
				<signInScript/>
			} }
		</Panel>
	</Layout>
}

// signInHeading is what the page is: signing in, or, by an invitation's link, what the invitation is to.
func signInHeading(ctx context.Context, v SignInView) string {
	if v.Invite != "" {
		return intl.Text(ctx, locales.ScreensInvited(v.InviteRole))
	}
	return intl.Text(ctx, locales.ScreensSignIn)
}

func signInAbout(ctx context.Context, v SignInView) string {
	if v.Invite != "" && !v.Caller.SignedIn() {
		return intl.Text(ctx, locales.ScreensInvitedIntro)
	}
	return ""
}

// passkeyButton signs in with a passkey the device has.
component passkeyButton(variant string) {
	<ui.Button type="button" id="sign-in-passkey" variant={variant}>
		<icon.KeyRound/>
		{ locales.ScreensSignInPasskey }
	</ui.Button>
}

// signUpForm makes an account: the name its person is shown by, and the button that has the device make a passkey.
component signUpForm(variant string) {
	<form id="sign-up" class="flex flex-col gap-2">
		<ui.Input
			name="name"
			id="sign-up-name"
			required
			maxlength="60"
			// What they are called here, for everyone to see: not what a browser fills in as a user name, and never
			// an address.
			autocomplete="nickname"
			pattern="[^@]*"
			title={locales.ScreensNameHint}
			aria-describedby="sign-up-name-hint"
			dir="auto"
			placeholder={locales.ScreensName}
			aria-label={locales.ScreensName}
		/>
		<p id="sign-up-name-hint" class="text-xs text-muted-foreground">{ locales.ScreensNameHint }</p>
		<ui.Button type="submit" variant={variant}>{ locales.ScreensSignUp }</ui.Button>
	</form>
}

// signInScript asks the device, as the two ceremonies have it: what the app says to ask (begin), the browser's own
// navigator.credentials, and what the device answered sent back (finish). Nothing here decides anything: the
// Worker checks every answer.
component signInScript() {
	<script>
		(function () {
			var box = document.getElementById("sign-in");
			var error = document.getElementById("sign-in-error");
			var post = function (path, body) {
				return fetch(path, { method: "POST", headers: { "Content-Type": "application/json" }, body: body ? JSON.stringify(body) : undefined }).then(function (response) {
					return response.ok ? response.json() : response.text().then(function (text) { throw new Error(text); });
				});
			};
			var invite = box.dataset.invite;
			var run = function (begin, finish, ask, body) {
				error.hidden = true;
				return post(begin, body).then(function (options) { return ask(options.publicKey); }).then(function (credential) {
					return post(finish, credential.toJSON());
				}).then(function () {
					// Signed in by an invitation's link, with an account they had: the invitation is theirs. (An
					// account made by the link was made with it.)
					if (invite && !body) {
						return fetch(box.dataset.accept, { method: "POST", body: new URLSearchParams({ invite: invite }) });
					}
				}).then(function () {
					// Invited, they land where it says what they are now.
					location.assign(invite ? box.dataset.account : box.dataset.return || "/");
				}).catch(function (e) {
					error.textContent = e && e.message && e.name !== "NotAllowedError" ? e.message : box.dataset.failed;
					error.hidden = false;
				});
			};
			document.getElementById("sign-in-passkey").addEventListener("click", function () {
				run(box.dataset.inBegin, box.dataset.inFinish, function (options) {
					return navigator.credentials.get({ publicKey: PublicKeyCredential.parseRequestOptionsFromJSON(options) });
				});
			});
			document.getElementById("sign-up").addEventListener("submit", function (event) {
				event.preventDefault();
				run(box.dataset.upBegin, box.dataset.upFinish, function (options) {
					return navigator.credentials.create({ publicKey: PublicKeyCredential.parseCreationOptionsFromJSON(options) });
				}, { name: event.target.elements.name.value, invite: box.dataset.invite });
			});
		})();
	</script>
}

// SignOutForm is the button that signs out: a form of its own, since it changes who is asking.
component SignOutForm() {
	<form method="post" action={SignOut{} |> url}>
		<ui.Button type="submit" variant="outline" size="sm" id="sign-out">{ locales.ScreensSignOut }</ui.Button>
	</form>
}

// AccountLink is who is signed in, for a header: their name, which leads to their account, or the way to sign in.
component AccountLink(attrs gsx.Attrs) {
	{{ who := api.Current(ctx) }}
	{ if who.SignedIn() {
		<ui.Button variant="ghost" size="sm" href={AccountPage{} |> url} id="account-link" { attrs... }>
			<icon.CircleUser/>
			<span dir="auto" translate="no">{ who.Name() }</span>
		</ui.Button>
	} else {
		<ui.Button variant="ghost" size="sm" href={SignInPage{} |> url} id="sign-in-link" { attrs... }>
			{ locales.ScreensSignIn }
		</ui.Button>
	} }
}

// Account is what is someone's own: who they are, signing out, and the tokens they make for their programs. Its
// Needs is why no one else gets here: whoever is not signed in is sent to sign in first.
type Account struct {
	page   AccountPage  `route:"GET /{$} Account"`
	create TokenCreate  `route:"POST /tokens TokenCreate"`
	revoke TokenRevoke  `route:"POST /tokens/{token}/revoke TokenRevoke"`
	leave  AccountLeave `route:"POST /leave AccountLeave"`
	people People       `route:"/people People"`
}

// Needs is what every route here needs: someone signed in.
func (Account) Needs() []api.Scope { return nil }

// AccountPage is the page.
type AccountPage struct{}

// AccountView is what it shows; Made is a token made a moment ago, shown this once.
type AccountView struct {
	Caller api.Caller
	Tokens []api.IssuedToken
	Made   string
	// MayManage is whether they may see everyone and change roles: the link to that page.
	MayManage bool
}

// Noindex keeps the page out of the sitemap and of search engines: it is one person's.
func (AccountPage) Noindex() bool { return true }

// FullLoad says a link to this page is a page load: its script runs as the document loads.
func (AccountPage) FullLoad() bool { return true }

func (AccountPage) Props(r *http.Request, caller api.Caller, auth *api.Auth) (AccountView, error) {
	tokens, err := auth.Tokens(r)
	return AccountView{Caller: caller, Tokens: tokens, MayManage: caller.May(People{}.Needs()...)}, err
}

// TokenCreate makes a token and answers with the list again, the new token shown once.
type TokenCreate struct{}

func (TokenCreate) ServeHTTP(w http.ResponseWriter, r *http.Request, caller api.Caller, auth *api.Auth) error {
	made, err := auth.NewToken(r, r.PostFormValue("name"))
	if err != nil {
		return err
	}
	tokens, err := auth.Tokens(r)
	if err != nil {
		return err
	}
	return structpages.RenderComponent(AccountPage{}.Tokens(AccountView{Caller: caller, Tokens: tokens, Made: made}))
}

// TokenRevoke takes a token away and answers with the list again.
type TokenRevoke struct{}

func (TokenRevoke) ServeHTTP(w http.ResponseWriter, r *http.Request, caller api.Caller, auth *api.Auth) error {
	id, err := strconv.ParseInt(r.PathValue("token"), 10, 64)
	if err != nil {
		return app.NotFound("no such token")
	}
	if err := auth.RevokeToken(r, id); err != nil {
		return err
	}
	tokens, err := auth.Tokens(r)
	if err != nil {
		return err
	}
	return structpages.RenderComponent(AccountPage{}.Tokens(AccountView{Caller: caller, Tokens: tokens}))
}

// AccountLeave deletes the account of whoever is signed in, with what the app keeps of theirs, and goes to the
// front page: a plain form's target.
type AccountLeave struct{}

func (AccountLeave) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) error {
	if err := auth.Leave(w, r); err != nil {
		return err
	}
	return app.Redirect{To: intl.URL(r.Context(), "/")}
}

component (p AccountPage) Page(v AccountView) {
	<Layout
		mode={LayoutPage}
		title={locales.ScreensAccount |> text}
		description={locales.ScreensTokensIntro |> text}
		heading={locales.ScreensAccount |> text}
		about={locales.ScreensSignedInAs(v.Caller.Name()) |> text}
	>
		<Panel hx-boost:inherited="false">
			<div class="flex flex-wrap items-center gap-2">
				// What they are, and with it what they may do.
				<ui.Badge id="role" translate="no">{ v.Caller.Role }</ui.Badge>
				{ for _, scope := range v.Caller.Scopes {
					<ui.Badge variant="secondary" translate="no">{ scope.String() }</ui.Badge>
				} }
				<span class="flex-1"></span>
				{ if v.MayManage {
					<ui.Button variant="outline" size="sm" href={PeoplePage{} |> url} id="people-link">
						{ locales.ScreensPeople }
					</ui.Button>
				} }
				<SignOutForm/>
			</div>
		</Panel>
		<p.Tokens v={v}/>
		// Leaving: asked twice, the second time with what goes, in gsxui's Dialog.
		<Panel id="leave" about={locales.ScreensAccountLeaveIntro |> text} hx-boost:inherited="false">
			<ui.Dialog>
				<div>
					<ui.Button
						variant="outline"
						size="sm"
						data-gsxui-slot-dialog-trigger
						aria-haspopup="dialog"
						aria-expanded="false"
					>
						{ locales.ScreensAccountLeave }
					</ui.Button>
				</div>
				<ui.DialogContent hideCloseButton={true}>
					<ui.DialogHeader>
						<ui.DialogTitle>{ locales.ScreensAccountLeave }</ui.DialogTitle>
						<ui.DialogDescription>{ locales.ScreensAccountLeaveIntro }</ui.DialogDescription>
					</ui.DialogHeader>
					<form method="post" action={AccountLeave{} |> url}>
						<ui.DialogFooter showCloseButton={false}>
							<ui.Button variant="outline" type="button" data-gsxui-dialog-close data-gsxui-slot-dialog-footer-close>
								{ locales.ScreensClose }
							</ui.Button>
							<ui.Button type="submit" variant="destructive">{ locales.ScreensAccountLeaveConfirm }</ui.Button>
						</ui.DialogFooter>
					</form>
				</ui.DialogContent>
			</ui.Dialog>
		</Panel>
	</Layout>
}

// Tokens is the tokens someone made for their programs, with the form that makes one: a region of the page that
// changes by itself. A token is shown once, when it is made; the app keeps only what lets it know the token again.
component (p AccountPage) Tokens(v AccountView) {
	<Panel id={AccountPage.Tokens |> id} title={locales.ScreensTokens |> text} about={locales.ScreensTokensIntro |> text}>
		{ if v.Made != "" {
			<Once label={locales.ScreensTokenOnce |> text} value={v.Made} id="token-made"/>
		} }
		<form
			class="flex flex-wrap gap-2"
			hx-post={TokenCreate{} |> url}
			hx-target={AccountPage.Tokens |> target}
			hx-swap="outerHTML"
		>
			<ui.Input
				name="name"
				class="flex-1"
				required
				maxlength="60"
				autocomplete="off"
				dir="auto"
				placeholder={locales.ScreensTokenName}
				aria-label={locales.ScreensTokenName}
			/>
			<ui.Button type="submit">{ locales.ScreensTokenNew }</ui.Button>
		</form>
		<Rows>
			{ for _, token := range v.Tokens {
				<Row>
					<span dir="auto" class="flex-1">{ token.Name }</span>
					{ for _, scope := range token.Scopes {
						<ui.Badge variant="secondary" translate="no">{ scope }</ui.Badge>
					} }
					<ui.Button
						variant="ghost"
						size="sm"
						type="button"
						hx-post={TokenRevoke{} |> url(map[string]any{"token": token.ID})}
						hx-target={AccountPage.Tokens |> target}
						hx-swap="outerHTML"
					>
						{ locales.ScreensTokenRevoke }
					</ui.Button>
				</Row>
			} }
		</Rows>
	</Panel>
}

// People is everyone who signed up, and what each is: for whoever may manage people. What a role may do is the
// app's to say (api.People's Roles); here a person is given one.
type People struct {
	page   PeoplePage   `route:"GET /{$} People"`
	set    RoleSet      `route:"POST /{person}/role RoleSet"`
	invite PeopleInvite `route:"POST /invite PeopleInvite"`
	remove PersonRemove `route:"POST /{person}/remove PersonRemove"`
}

// Needs is what every route here needs: the scope of whoever runs the app. Someone signed in without it gets a 403.
func (People) Needs() []api.Scope { return []api.Scope{api.Admin} }

// PeoplePage is the page.
type PeoplePage struct{}

// FullLoad says a link to this page is a page load, as the other pages of this screen are.
func (PeoplePage) FullLoad() bool { return true }

func (PeoplePage) Noindex() bool { return true }

// PeopleView is what it shows: everyone, and the roles there are.
type PeopleView struct {
	Caller api.Caller
	People []api.Person
	Roles  []string
}

func (PeoplePage) Props(r *http.Request, caller api.Caller, auth *api.Auth) (PeopleView, error) {
	people, err := auth.Everyone(r)
	return PeopleView{Caller: caller, People: people, Roles: auth.Roles()}, err
}

// RoleSet gives a person a role and answers with their row again.
type RoleSet struct{}

func (RoleSet) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) error {
	id, role := r.PathValue("person"), r.PostFormValue("role")
	if err := auth.SetRole(r, id, role); err != nil {
		return err
	}
	return structpages.RenderComponent(PeoplePage{}.Person(api.Person{
		ID:   id,
		Name: r.PostFormValue("name"),
		Role: role,
	}, auth.Roles(), false))
}

// PersonRemove takes a person away, with what the app keeps of theirs, and answers with nothing: their row goes.
type PersonRemove struct{}

func (PersonRemove) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) error {
	if err := auth.Remove(r, r.PathValue("person")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusOK)
	return nil
}

// PeopleInvite makes an invitation to a role and answers with its link, shown this once: whoever signs up through
// it is that. The same thing charter run people:invite makes for the first admin.
type PeopleInvite struct{}

func (PeopleInvite) ServeHTTP(w http.ResponseWriter, r *http.Request, auth *api.Auth) error {
	code, err := auth.Invite(r, r.PostFormValue("role"))
	if err != nil {
		return err
	}
	link := api.Origin(r) + app.InvitePath + "?code=" + code
	return structpages.RenderComponent(PeoplePage{}.Invite(auth.Roles(), link))
}

component (p PeoplePage) Page(v PeopleView) {
	<Layout
		mode={LayoutPage}
		title={locales.ScreensPeople |> text}
		description={locales.ScreensPeopleIntro |> text}
		heading={locales.ScreensPeople |> text}
		about={locales.ScreensPeopleIntro |> text}
		tabs={peopleTabs()}
	>
		<Panel hx-boost:inherited="false">
			<Rows id="people">
				{ for _, person := range v.People {
					<p.Person person={person} roles={v.Roles} mine={person.ID == v.Caller.Subject}/>
				} }
			</Rows>
		</Panel>
		<p.Invite roles={v.Roles} link=""/>
	</Layout>
}

// PeopleTabs is the way between the pages this one is one of, for an app that has them: the admin screen says it
// (its own tabs), since who is what is an admin's to say. nil: the page stands by itself.
var PeopleTabs func() gsx.Node

func peopleTabs() gsx.Node {
	if PeopleTabs == nil {
		return nil
	}
	return PeopleTabs()
}

// Invite is the form that makes an invitation, and the link of one made a moment ago: a region of the page that
// changes by itself.
component (p PeoplePage) Invite(roles []string, link string) {
	<Panel id={PeoplePage.Invite |> id} title={locales.ScreensInvite |> text} about={locales.ScreensInviteIntro |> text}>
		{ if link != "" {
			<Once label={locales.ScreensInviteOnce |> text} value={link} id="invite-link">
				// The device's own share sheet where it has one (a phone: Messages, Mail, AirDrop), else the
				// clipboard, and the button says so.
				<ui.Button
					type="button"
					variant="outline"
					size="sm"
					id="invite-share"
					data-link={link}
					data-copied={locales.ScreensInviteCopied |> text}
					hx-on:click=js`var b = this, link = b.dataset.link; if (navigator.share) { navigator.share({ url: link }).catch(function () {}); } else { navigator.clipboard.writeText(link).then(function () { b.lastElementChild.textContent = b.dataset.copied; }); }`
				>
					<icon.Share/>
					<span>{ locales.ScreensInviteShare }</span>
				</ui.Button>
			</Once>
		} }
		<form
			class="flex flex-wrap gap-2"
			hx-post={PeopleInvite{} |> url}
			hx-target={PeoplePage.Invite |> target}
			hx-swap="outerHTML"
		>
			<ui.NativeSelect name="role" aria-label={locales.ScreensRole}>
				{ for _, role := range roles {
					<ui.NativeSelectOption value={role}>{ role }</ui.NativeSelectOption>
				} }
			</ui.NativeSelect>
			<ui.Button type="submit">{ locales.ScreensInviteNew }</ui.Button>
		</form>
	</Panel>
}

// Person is one person and their role, which changes as it is chosen: the row asks for itself again (its id is
// the component's, keyed by the person).
component (p PeoplePage) Person(person api.Person, roles []string, mine bool) {
	<Row id={PeoplePage.Person |> key(person.ID)}>
		<span dir="auto" class="flex-1" translate="no">{ person.Name }</span>
		<form
			hx-post={RoleSet{} |> url(map[string]any{"person": person.ID})}
			hx-target={PeoplePage.Person |> keytarget(person.ID)}
			hx-swap="outerHTML"
			hx-trigger="change"
		>
			<input type="hidden" name="name" value={person.Name}/>
			<ui.NativeSelect name="role" aria-label={locales.ScreensRole}>
				{ for _, role := range roles {
					<ui.NativeSelectOption value={role} selected={role == person.Role}>{ role }</ui.NativeSelectOption>
				} }
			</ui.NativeSelect>
		</form>
		// Anyone but oneself, asked first: one's own account is deleted on one's account page.
		{ if !mine {
			<ui.Button
				type="button"
				variant="ghost"
				size="icon"
				class="size-8"
				data-remove
				aria-label={locales.ScreensPersonRemove(person.Name)}
				title={locales.ScreensPersonRemove(person.Name)}
				hx-post={PersonRemove{} |> url(map[string]any{"person": person.ID})}
				hx-confirm={locales.ScreensPersonRemoveConfirm(person.Name)}
				hx-target={PeoplePage.Person |> keytarget(person.ID)}
				hx-swap="delete"
			>
				<icon.Trash2/>
			</ui.Button>
		} }
	</Row>
}
