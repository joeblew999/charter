package apptest

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/webauthn"
	"github.com/chromedp/chromedp"
)

// Page is a tab of a real browser (Chrome, headless) on the Worker under test: what a reader sees and does. A test of
// a page is written with it, so that htmx, the stream and the scripts Vite built are part of what is tested.
//
//	home := apptest.Browser(t)
//	home.Open("/")
//	home.Type(`#post input[name=body]`, "hello")
//	home.Click(`#post button`)
//	home.WaitText("#notes", "hello")
type Page struct {
	t   testing.TB
	ctx context.Context
}

// wait is how long a page has to show what a test waits for.
const wait = 20 * time.Second

// Browser starts Chrome and gives its first tab. It is closed with the test. A machine without Chrome fails the
// test with what is missing: a page is not tested by leaving its test out.
func Browser(t testing.TB) *Page {
	t.Helper()
	Base(t)
	options := append(chromedp.DefaultExecAllocatorOptions[:], chromedp.NoSandbox)
	alloc, cancelAlloc := chromedp.NewExecAllocator(context.Background(), options...)
	ctx, cancel := chromedp.NewContext(alloc)
	t.Cleanup(func() { cancel(); cancelAlloc() })
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[string](`""`)); err != nil {
		t.Fatalf("no browser: %v (Chrome or Chromium must be installed)", err)
	}
	p := &Page{t, ctx}
	p.authenticator()
	return p
}

// authenticator gives the tab a device that holds passkeys and answers for its owner at once: Chrome's own virtual
// authenticator. A page that asks for a passkey (the sign-in screen) then gets one with no person and no device.
func (p *Page) authenticator() {
	p.t.Helper()
	err := chromedp.Do(p.ctx, chromedp.Func(func(ctx context.Context, _ *chromedp.Target) error {
		if _, err := chromedp.Call(ctx, webauthn.Enable, webauthn.EnableParams{}); err != nil {
			return err
		}
		_, err := chromedp.Call(ctx, webauthn.AddVirtualAuthenticator, webauthn.AddVirtualAuthenticatorParams{Options: &webauthn.VirtualAuthenticatorOptions{
			Protocol: "ctap2", Transport: "internal",
			HasResidentKey: true, HasUserVerification: true, IsUserVerified: true, AutomaticPresenceSimulation: true,
		}})
		return err
	}))
	if err != nil {
		p.t.Fatalf("no virtual authenticator: %v", err)
	}
}

// SignUp makes an account on the app's sign-in screen (charter add screen:sign-in), at its path (with ?invite= and a
// code, as an invitation's link leads there), under a name, and
// waits until the tab is signed in: the passkey is made by the tab's virtual authenticator. It is how a test of a
// page for someone signed in gets there, as a reader would.
func (p *Page) SignUp(signIn, name string) {
	p.t.Helper()
	p.Open(signIn)
	// The page's own path: an invitation's link has its code after it.
	signIn, _, _ = strings.Cut(signIn, "?")
	p.Type("#sign-up-name", name)
	// A real click: the browser makes a passkey only for something its user did.
	ctx, cancel := context.WithTimeout(p.ctx, wait)
	defer cancel()
	if err := chromedp.Do(ctx, chromedp.Click(`#sign-up button[type=submit]`)); err != nil {
		p.t.Fatalf("sign up: %v", err)
	}
	for start := time.Now(); time.Since(start) < wait; time.Sleep(50 * time.Millisecond) {
		if at, ok := p.try(`location.pathname`); ok && !strings.HasSuffix(strings.TrimSuffix(at, "/"), strings.TrimSuffix(signIn, "/")) {
			// Whoever a test makes, it takes away again: the Worker it asks, a deployed one too, is left as it was.
			p.t.Cleanup(func() { p.leave(signIn, name) })
			return
		}
	}
	p.t.Fatalf("signing up as %s never left the sign-in page; it says: %s", name, p.Text("#sign-in-error"))
}

// Leave deletes the account the tab is signed up with when the test is done, for a test that signed up by hand
// (SignUp does this itself).
func (p *Page) Leave(signIn string) {
	p.t.Cleanup(func() { p.leave(signIn, "by hand") })
}

// leave deletes the account the tab signed up with, on the account page's own form (the sign-in screen's), signing
// in again first when the test signed out. With the account goes what the app keeps of that person.
func (p *Page) leave(signIn, name string) {
	ask := func(expression string) string { v, _ := p.try(expression); return v }
	open := func(address string) bool {
		ctx, cancel := context.WithTimeout(p.ctx, wait)
		defer cancel()
		_, err := chromedp.Run(ctx, chromedp.Navigate(address))
		return err == nil
	}
	loaded := func(selector string) bool {
		for start := time.Now(); time.Since(start) < wait; time.Sleep(50 * time.Millisecond) {
			if ask(fmt.Sprintf(`String(!!document.querySelector(%q) && document.readyState === "complete")`, selector)) == "true" {
				return true
			}
		}
		return false
	}
	failed := func(why string) { p.t.Errorf("the account made for this test (%s) was not deleted: %s", name, why) }
	if !open(Base(p.t)+signIn) || !loaded("#signed-in, #sign-in-passkey") {
		failed("the sign-in page did not open")
		return
	}
	if ask(`String(!!document.querySelector("#signed-in"))`) != "true" {
		ctx, cancel := context.WithTimeout(p.ctx, wait)
		err := chromedp.Do(ctx, chromedp.Click(`#sign-in-passkey`))
		cancel()
		// Signed in, it leaves the sign-in page. Refused, the page says so: the test deleted the account itself,
		// and there is nothing left to take away.
		for start := time.Now(); err == nil; time.Sleep(50 * time.Millisecond) {
			if at := ask(`location.pathname`); at != "" && !strings.HasSuffix(strings.TrimSuffix(at, "/"), strings.TrimSuffix(signIn, "/")) {
				break
			}
			if strings.TrimSpace(ask(`document.querySelector("#sign-in-error")?.textContent ?? ""`)) != "" {
				return
			}
			if time.Since(start) > wait {
				err = context.DeadlineExceeded
			}
		}
		if err != nil || !open(Base(p.t)+signIn) || !loaded("#signed-in") {
			failed("it could not sign in again")
			return
		}
	}
	account := ask(`document.querySelector("#account").href`)
	if account == "" || !open(account) || !loaded("#leave form") {
		failed("its account page has no way to delete it")
		return
	}
	ask(`document.querySelector("#leave form").submit(), ""`)
	for start := time.Now(); time.Since(start) < wait; time.Sleep(50 * time.Millisecond) {
		if ask(`String(!document.querySelector("#leave") && document.readyState === "complete")`) == "true" {
			return
		}
	}
	failed("the form did not answer")
}

// SignIn signs in on the sign-in screen with the passkey the tab's authenticator has (after SignUp and signing
// out), and waits until the tab is signed in.
func (p *Page) SignIn(signIn string) {
	p.t.Helper()
	p.Open(signIn)
	p.until("#sign-in-passkey")
	ctx, cancel := context.WithTimeout(p.ctx, wait)
	defer cancel()
	if err := chromedp.Do(ctx, chromedp.Click(`#sign-in-passkey`)); err != nil {
		p.t.Fatalf("sign in: %v", err)
	}
	for start := time.Now(); time.Since(start) < wait; time.Sleep(50 * time.Millisecond) {
		if at, ok := p.try(`location.pathname`); ok && !strings.HasSuffix(strings.TrimSuffix(at, "/"), strings.TrimSuffix(signIn, "/")) {
			return
		}
	}
	p.t.Fatalf("signing in never left the sign-in page; it says: %s", p.Text("#sign-in-error"))
}

// Tab is another tab of the same browser: a second reader.
func (p *Page) Tab() *Page {
	p.t.Helper()
	ctx, cancel := chromedp.NewContext(p.ctx)
	p.t.Cleanup(cancel)
	if _, err := chromedp.Run(ctx, chromedp.Evaluate[string](`""`)); err != nil {
		p.t.Fatal(err)
	}
	return &Page{p.t, ctx}
}

// js evaluates an expression in the page and gives its value as text.
func (p *Page) js(what, expression string) string {
	p.t.Helper()
	ctx, cancel := context.WithTimeout(p.ctx, wait)
	defer cancel()
	value, err := chromedp.Run(ctx, chromedp.Evaluate[string](expression))
	if err != nil {
		p.t.Fatalf("%s: %v", what, err)
	}
	return value
}

// try is js for a page that may be between two documents (a click that navigates): no value is not a failure, and
// whoever waits asks again.
func (p *Page) try(expression string) (string, bool) {
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()
	value, err := chromedp.Run(ctx, chromedp.Evaluate[string](expression))
	return value, err == nil
}

// until waits for an element a CSS selector names to be in the page.
func (p *Page) until(selector string) {
	p.t.Helper()
	for start := time.Now(); time.Since(start) < wait; time.Sleep(50 * time.Millisecond) {
		// There, on a page that has loaded, scripts and all: what a reader does next finds htmx listening.
		if found, _ := p.try(fmt.Sprintf(`String(!!document.querySelector(%q) && document.readyState === "complete")`, selector)); found == "true" {
			return
		}
	}
	p.t.Fatalf("the page has no %s", selector)
}

// Open goes to a path of the Worker and waits for the page to load.
func (p *Page) Open(path string) {
	p.t.Helper()
	ctx, cancel := context.WithTimeout(p.ctx, wait)
	defer cancel()
	if _, err := chromedp.Run(ctx, chromedp.Navigate(Base(p.t)+path)); err != nil {
		p.t.Fatalf("open %s: %v", path, err)
	}
	p.until("body")
}

// Type puts text into the field a CSS selector names, as typing does: the field's value, then its input event.
func (p *Page) Type(selector, text string) {
	p.t.Helper()
	p.until(selector)
	p.js("type into "+selector, fmt.Sprintf(`(() => { const e = document.querySelector(%q); e.focus(); e.value = %q; e.dispatchEvent(new Event("input", { bubbles: true })); return "" })()`, selector, text))
}

// Click clicks the element a CSS selector names. A form's submit button submits the form, as a reader's click does.
func (p *Page) Click(selector string) {
	p.t.Helper()
	p.until(selector)
	p.js("click "+selector, fmt.Sprintf(`(() => { document.querySelector(%q).click(); return "" })()`, selector))
}

// Text is the text of the element a CSS selector names.
func (p *Page) Text(selector string) string {
	p.t.Helper()
	p.until(selector)
	return p.js("read "+selector, fmt.Sprintf(`document.querySelector(%q).textContent ?? ""`, selector))
}

// Attr is an attribute of the element a CSS selector names, "" without it.
func (p *Page) Attr(selector, name string) string {
	p.t.Helper()
	p.until(selector)
	return p.js("read "+selector, fmt.Sprintf(`document.querySelector(%q).getAttribute(%q) ?? ""`, selector, name))
}

// Select chooses the option with that value in the menu a CSS selector names, as a reader does: its change event
// follows.
func (p *Page) Select(selector, value string) {
	p.t.Helper()
	p.until(selector)
	p.js("choose in "+selector, fmt.Sprintf(`(() => { const e = document.querySelector(%q); e.value = %q; e.dispatchEvent(new Event("change", { bubbles: true })); return "" })()`, selector, value))
}

// Has reports whether the page has an element a CSS selector names, now: for what must not be there.
func (p *Page) Has(selector string) bool {
	p.t.Helper()
	return p.js("look for "+selector, fmt.Sprintf(`String(!!document.querySelector(%q))`, selector)) == "true"
}

// Path is the path of the page the tab is on.
func (p *Page) Path() string {
	p.t.Helper()
	return p.js("read the address", `location.pathname`)
}

// WaitText waits until the element a CSS selector names contains text, and fails the test with what it shows instead
// when it does not in time.
func (p *Page) WaitText(selector, text string) {
	p.t.Helper()
	var got string
	for start := time.Now(); time.Since(start) < wait; time.Sleep(100 * time.Millisecond) {
		if now, ok := p.try(fmt.Sprintf(`document.querySelector(%q)?.textContent ?? ""`, selector)); ok {
			if got = now; strings.Contains(got, text) {
				return
			}
		}
	}
	p.t.Fatalf("%s never showed %q; it shows:\n%s", selector, text, strings.TrimSpace(got))
}
