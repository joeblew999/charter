package cloud

// What a Worker is bound to, by name, as values with types.
//
// A binding's name is declared once, in wrangler.jsonc (a variable's there too, a secret's in .dev.vars.example
// and fnox.toml). In Go it was a string: cloud.Open("DB"), kv.NewNamespace("KV"), which compiles whatever it says
// and fails on a Worker when nothing is bound there, or something of another kind is. So here each kind is a type,
// and charter writes an app's bindings as values of them, from those files (env/env.gen.go: env.DB, env.KV,
// env.APP_NAME). A binding that is not declared is then a name that does not exist, and a bucket handed to what
// takes a database does not compile.
//
// No value of these types can be written by hand: their fields are unexported, and the Bound… functions that make
// them are for the generated file alone (charter check fails an app that calls one anywhere else, as it fails one
// that calls workers-go's own kv.NewNamespace, r2.NewBucket, queues.NewProducer or cloudflare.Getenv with a name).
//
// This is what workers-go lacks, and so charter's: its packages take a binding's name as a string.

// Database is a D1 database (wrangler.jsonc's d1_databases): Open is it, for the queries.
type Database struct{ name string }

// KV is a KV namespace (kv_namespaces): Open is workers-go's *kv.Namespace.
type KV struct{ name string }

// Bucket is an R2 bucket (r2_buckets): Open is workers-go's *r2.Bucket.
type Bucket struct{ name string }

// Queue is a queue the Worker sends to (queues.producers): Open is workers-go's *queues.Producer.
type Queue struct{ name string }

// Object is a Durable Object namespace (durable_objects.bindings): what a live.Feed's hub is in.
type Object struct{ name string }

// RateLimit is a Rate Limiting binding (ratelimits), with what wrangler.jsonc says it allows: Calls in each span of
// Seconds. Allow asks it.
type RateLimit struct {
	name           string
	calls, seconds int
}

// Mailer is a send_email binding: Send sends through it.
type Mailer struct{ name string }

// Variable is a variable of the Worker (wrangler.jsonc's vars) or a secret (.dev.vars.example, fnox.toml): Value
// is what it holds, "" when it is not set.
type Variable struct{ name string }

// The Bound… functions make a binding's value from its name. They are for env/env.gen.go, which charter writes
// from wrangler.jsonc, and for charter's own code that is handed a name by the Worker as it runs (the admin
// screen's list of what is bound).

func BoundDatabase(name string) Database { return Database{name} }
func BoundKV(name string) KV             { return KV{name} }
func BoundBucket(name string) Bucket     { return Bucket{name} }
func BoundQueue(name string) Queue       { return Queue{name} }
func BoundObject(name string) Object     { return Object{name} }
func BoundMailer(name string) Mailer     { return Mailer{name} }
func BoundVariable(name string) Variable { return Variable{name} }

// BoundRateLimit is a Rate Limiting binding that allows calls in each span of seconds.
func BoundRateLimit(name string, calls, seconds int) RateLimit {
	return RateLimit{name, calls, seconds}
}

// Name is the binding's name, as wrangler.jsonc has it.
func (b Database) Name() string  { return b.name }
func (b KV) Name() string        { return b.name }
func (b Bucket) Name() string    { return b.name }
func (b Queue) Name() string     { return b.name }
func (b Object) Name() string    { return b.name }
func (b RateLimit) Name() string { return b.name }
func (b Mailer) Name() string    { return b.name }
func (b Variable) Name() string  { return b.name }

// Calls and Seconds are what the binding allows a caller: at most Calls in each span of Seconds.
func (b RateLimit) Calls() int   { return b.calls }
func (b RateLimit) Seconds() int { return b.seconds }

// Open is the database, for the queries. On a Worker that has no such binding it is an error that says so.
func (b Database) Open() (DB, error) { return open(b.name) }

// Allow asks the binding whether key may make one more call. Its counter is local to the Cloudflare location, so
// it answers without a network round trip. A missing binding allows, with ErrNoBinding: it never takes an app down.
func (b RateLimit) Allow(key string) (bool, error) { return allow(b.name, key) }

// Send sends raw, a whole message as RFC 5322 has it (headers, a blank line, the body), from one address to
// another. Cloudflare delivers only to addresses the binding allows.
func (b Mailer) Send(from, to string, raw []byte) error { return sendEmail(b.name, from, to, raw) }

// Value is what the variable or the secret holds, "" when it is not set.
func (b Variable) Value() string { return Var(b.name) }
