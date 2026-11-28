package app

// Scope is one thing a caller may do, by its word: "write". Who may do what is said in scopes: a role grants some,
// a page and an operation of the API need some, and the same code compares them (api.Auth).
//
// A scope is a value, declared once where the app says who may do what, and named everywhere else:
//
//	var (
//		Read  = charter.NewScope("read")
//		Write = charter.NewScope("write")
//	)
//
//	func (PostNote) Needs() []charter.Scope { return []charter.Scope{api.Write} }
//
// It is not a string, so that it cannot be written as one: a word typed where a scope is needed
// (Needs: "wrtie") would compile, name a scope no role grants, and shut everyone out of the page with nothing to
// say why. A scope that is not declared is a name that does not exist. charter check fails an app that makes one
// anywhere but in a declaration (its part scopes).
type Scope struct{ word string }

// NewScope declares a scope: once, as a package's variable.
func NewScope(word string) Scope { return Scope{word} }

// String is the scope's word, as a token's and the API's specs have it.
func (s Scope) String() string { return s.word }

// MarshalText and UnmarshalText are the scope as its word, wherever one is written out or read back.
func (s Scope) MarshalText() ([]byte, error) { return []byte(s.word), nil }

func (s *Scope) UnmarshalText(b []byte) error {
	s.word = string(b)
	return nil
}

// Words are scopes as their words.
func Words(scopes []Scope) []string {
	words := make([]string, len(scopes))
	for i, s := range scopes {
		words[i] = s.word
	}
	return words
}
