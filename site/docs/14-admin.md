# The admin screen

What an admin sees of the system: the app at a glance, what the Worker is bound to, and what its databases hold. It is one of charter's [screens](12-screens-and-themes.md), for an app that has [people who sign in](13-sign-in.md).

```sh
go tool charter add screen:admin
```

```go
admin screens.Admin `route:"/admin Admin"`
```

```gsx
<screens.AdminLink/>
```

The link goes in your header. It is there for whoever may open the pages, and for no one else. The pages need the scope that manages people (`admin`): nobody is sent to sign in, and someone signed in without it gets a 403, by the same rule as every other page.

## The pages

| Page | Shows |
|---|---|
| Overview | The app's name; the charter and Go versions it was built with, and its own commit; how many people there are of each role; the invitations that still wait, each with a button that cancels it; the app's own facts |
| People | The [people page](13-sign-in.md#the-first-admin-and-inviting-people): roles, invitations, removing a person |
| System | Each binding the Worker has, and what kind it is; its variables and secrets, by name, with whether each is set |
| Database | Each database's tables with how many rows they have, and the migrations that ran |

Each page asks only for what it shows. Nothing here changes anything, but an invitation that is cancelled.

**It is what the Worker really has.** The bindings are read from the Worker's own environment as it runs, not from `wrangler.jsonc`: what is listed is what is bound. A database, a KV namespace and a bucket have a Check button, which asks one small thing (a `SELECT 1`, a key that is not there) and says whether it answered and how fast. It asks when you press it, never when the page loads.

**A value is never shown.** A variable and a secret are listed by name, with "set" or "not set". An invitation is listed by what it is to and when it ends, never by its code, which is the invitation.

## The app's own facts

What an admin wants to know of your app goes on the overview, as lines you name:

```go
(&app.App{
	Facts: []app.Fact{
		{Label: locales.FactsNotes, Value: func(ctx context.Context) (any, error) {
			report, err := api.Report(ctx)
			return report.Notes, err
		}},
	},
}).Run()
```

Each is asked when the page is. A number is written as the reader's language writes one, a `time.Time` as a date, anything else as it prints.

## From a program

The pages are functions of `api.Auth`, which check the caller as the pages do: `auth.Overview(r)`, `auth.System(r)`, `auth.Probe(r, binding)`, `auth.Databases(r)`, `auth.CancelInvitation(r, id)`. Their answers are plain values with JSON names, so an operation of your contract can give a program the same thing a page gives a person.

## What it does not do

- **Logs and errors.** A Worker keeps none itself: `go tool charter run logs` follows them live.
- **Anything on Cloudflare.** Deleting a resource and changing a secret are commands ([deploying](09-deploy.md)), each asked twice.
