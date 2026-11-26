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

// SignUp makes an account on the app's sign-in screen (charter add screen:sign-in), at its path, under a name, and
// waits until the tab is signed in: the passkey is made by the tab's virtual authenticator. It is how a test of a
// page for someone signed in gets there, as a reader would.
func (p *Page) SignUp(signIn, name string) {
	p.t.Helper()
	p.Open(signIn)
	p.Type("#sign-up-name", name)
	// A real click: the browser makes a passkey only for something its user did.
	ctx, cancel := context.WithTimeout(p.ctx, wait)
	defer cancel()
	if err := chromedp.Do(ctx, chromedp.Click(`#sign-up button[type=submit]`)); err != nil {
		p.t.Fatalf("sign up: %v", err)
	}
	for start := time.Now(); time.Since(start) < wait; time.Sleep(50 * time.Millisecond) {
		if at, ok := p.try(`location.pathname`); ok && !strings.HasSuffix(strings.TrimSuffix(at, "/"), strings.TrimSuffix(signIn, "/")) {
			return
		}
	}
	p.t.Fatalf("signing up as %s never left the sign-in page; it says: %s", name, p.Text("#sign-in-error"))
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
		if found, _ := p.try(fmt.Sprintf(`String(!!document.querySelector(%q))`, selector)); found == "true" {
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
