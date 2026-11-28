// Code generated from schema.sql and queries.sql, with sqlc, as an app's db/queries.gen.go is. DO NOT EDIT.
// Change those, then UPDATE=1 go test ./internal/accounts.

package accounts

import "github.com/joeblew999/charter/cloud"

// PersonByTokenRow is a row of PersonByToken.
type PersonByTokenRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Scopes string `json:"scopes"`
	Kind   string `json:"kind"`
}

// PersonRow is a row of Person.
type PersonRow struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// EveryoneRow is a row of Everyone.
type EveryoneRow struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

// PasskeysRow is a row of Passkeys.
type PasskeysRow struct {
	Credential string `json:"credential"`
}

// TokensRow is a row of Tokens.
type TokensRow struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Scopes    string `json:"scopes"`
	CreatedAt string `json:"created_at"`
}

// StartedRow is a row of Started.
type StartedRow struct {
	Data string `json:"data"`
}

// FinishRow is a row of Finish.
type FinishRow struct {
	Data string `json:"data"`
}

// CeremoniesRow is a row of the table ceremonies.
type CeremoniesRow struct {
	ID        string `json:"id"`
	Data      string `json:"data"`
	ExpiresAt string `json:"expires_at"`
}

const qPersonByToken = "SELECT users.id, users.name, users.role, tokens.scopes, tokens.kind FROM tokens JOIN users ON users.id = tokens.user_id WHERE tokens.hash = ? AND (tokens.expires_at IS NULL OR tokens.expires_at > CURRENT_TIMESTAMP)"

// PersonByToken is whoever a token's hash is, with what the token is and may do, while it has not expired.
func PersonByToken(db cloud.DB, hash string) (PersonByTokenRow, error) {
	return cloud.One[PersonByTokenRow](db, qPersonByToken, hash)
}

const qPerson = "SELECT id, name, role FROM users WHERE id = ?"

// Person is one person, by their id.
func Person(db cloud.DB, id string) (PersonRow, error) {
	return cloud.One[PersonRow](db, qPerson, id)
}

const qCreatePerson = "INSERT INTO users (id, name, role) VALUES (?, ?, ?)"

// CreatePerson keeps a new person.
func CreatePerson(db cloud.DB, id string, name string, role string) error {
	return cloud.Exec(db, qCreatePerson, id, name, role)
}

// CreatePersonStmt is CreatePerson as a statement of a cloud.Batch.
func CreatePersonStmt(id string, name string, role string) cloud.Statement {
	return cloud.Stmt(qCreatePerson, id, name, role)
}

const qEveryone = "SELECT id, name, role FROM users ORDER BY created_at, id LIMIT 500"

// Everyone is the app's people, the first to sign up first.
func Everyone(db cloud.DB) ([]EveryoneRow, error) {
	return cloud.Query[EveryoneRow](db, qEveryone)
}

const qSetRole = "UPDATE users SET role = ? WHERE id = ?"

// SetRole makes a person what role says, and says whether there was such a person.
func SetRole(db cloud.DB, role string, id string) (int64, error) {
	return cloud.ExecRows(db, qSetRole, role, id)
}

// SetRoleStmt is SetRole as a statement of a cloud.Batch.
func SetRoleStmt(role string, id string) cloud.Statement {
	return cloud.Stmt(qSetRole, role, id)
}

const qRemoveTokensOf = "DELETE FROM tokens WHERE user_id = ?"

// RemoveTokensOf takes away every session and token someone holds.
func RemoveTokensOf(db cloud.DB, userID string) error {
	return cloud.Exec(db, qRemoveTokensOf, userID)
}

// RemoveTokensOfStmt is RemoveTokensOf as a statement of a cloud.Batch.
func RemoveTokensOfStmt(userID string) cloud.Statement {
	return cloud.Stmt(qRemoveTokensOf, userID)
}

const qRemovePasskeysOf = "DELETE FROM passkeys WHERE user_id = ?"

// RemovePasskeysOf takes away every passkey someone signs in with.
func RemovePasskeysOf(db cloud.DB, userID string) error {
	return cloud.Exec(db, qRemovePasskeysOf, userID)
}

// RemovePasskeysOfStmt is RemovePasskeysOf as a statement of a cloud.Batch.
func RemovePasskeysOfStmt(userID string) cloud.Statement {
	return cloud.Stmt(qRemovePasskeysOf, userID)
}

const qRemovePerson = "DELETE FROM users WHERE id = ?"

// RemovePerson takes a person away: after their tokens and passkeys, in one batch.
func RemovePerson(db cloud.DB, id string) error {
	return cloud.Exec(db, qRemovePerson, id)
}

// RemovePersonStmt is RemovePerson as a statement of a cloud.Batch.
func RemovePersonStmt(id string) cloud.Statement {
	return cloud.Stmt(qRemovePerson, id)
}

const qPasskeys = "SELECT credential FROM passkeys WHERE user_id = ?"

// Passkeys is the passkeys someone signs in with.
func Passkeys(db cloud.DB, userID string) ([]string, error) {
	rows, err := cloud.Query[PasskeysRow](db, qPasskeys, userID)
	values := make([]string, len(rows))
	for i, row := range rows {
		values[i] = row.Credential
	}
	return values, err
}

const qAddPasskey = "INSERT INTO passkeys (id, user_id, credential) VALUES (?, ?, ?) ON CONFLICT (id) DO UPDATE SET credential = excluded.credential"

// AddPasskey keeps a passkey, or what it is now when it is kept already.
func AddPasskey(db cloud.DB, id string, userID string, credential string) error {
	return cloud.Exec(db, qAddPasskey, id, userID, credential)
}

// AddPasskeyStmt is AddPasskey as a statement of a cloud.Batch.
func AddPasskeyStmt(id string, userID string, credential string) cloud.Statement {
	return cloud.Stmt(qAddPasskey, id, userID, credential)
}

const qIssueToken = "INSERT INTO tokens (hash, user_id, kind, name, scopes, expires_at) VALUES (?, ?, ?, ?, ?, ?)"

// IssueToken keeps a session or a token by its hash, never itself.
func IssueToken(db cloud.DB, hash string, userID string, kind string, name string, scopes string, expiresAt *string) error {
	return cloud.Exec(db, qIssueToken, hash, userID, kind, name, scopes, cloud.OrNull(expiresAt))
}

// IssueTokenStmt is IssueToken as a statement of a cloud.Batch.
func IssueTokenStmt(hash string, userID string, kind string, name string, scopes string, expiresAt *string) cloud.Statement {
	return cloud.Stmt(qIssueToken, hash, userID, kind, name, scopes, cloud.OrNull(expiresAt))
}

const qTokens = "SELECT id, name, scopes, created_at FROM tokens WHERE user_id = ? AND kind = ? ORDER BY id DESC"

// Tokens is the tokens of one kind someone holds, newest first.
func Tokens(db cloud.DB, userID string, kind string) ([]TokensRow, error) {
	return cloud.Query[TokensRow](db, qTokens, userID, kind)
}

const qRevokeToken = "DELETE FROM tokens WHERE user_id = ? AND (id = ? OR hash = ?)"

// RevokeToken takes away one of someone's tokens, by its id or by its hash.
func RevokeToken(db cloud.DB, userID string, id int64, hash string) error {
	return cloud.Exec(db, qRevokeToken, userID, id, hash)
}

// RevokeTokenStmt is RevokeToken as a statement of a cloud.Batch.
func RevokeTokenStmt(userID string, id int64, hash string) cloud.Statement {
	return cloud.Stmt(qRevokeToken, userID, id, hash)
}

const qForgetStarted = "DELETE FROM ceremonies WHERE expires_at < CURRENT_TIMESTAMP"

// ForgetStarted takes away what was started and never finished.
func ForgetStarted(db cloud.DB) error {
	return cloud.Exec(db, qForgetStarted)
}

// ForgetStartedStmt is ForgetStarted as a statement of a cloud.Batch.
func ForgetStartedStmt() cloud.Statement {
	return cloud.Stmt(qForgetStarted)
}

const qStart = "INSERT INTO ceremonies (id, data, expires_at) VALUES (?, ?, ?)"

// Start keeps something started (a sign-in, an invitation) until it is finished or expires.
func Start(db cloud.DB, id string, data string, expiresAt string) error {
	return cloud.Exec(db, qStart, id, data, expiresAt)
}

// StartStmt is Start as a statement of a cloud.Batch.
func StartStmt(id string, data string, expiresAt string) cloud.Statement {
	return cloud.Stmt(qStart, id, data, expiresAt)
}

const qStarted = "SELECT data FROM ceremonies WHERE id = ? AND expires_at > CURRENT_TIMESTAMP"

// Started is what was started under an id, while it has not expired.
func Started(db cloud.DB, id string) (string, error) {
	row, err := cloud.One[StartedRow](db, qStarted, id)
	return row.Data, err
}

const qFinish = "DELETE FROM ceremonies WHERE id = ? AND expires_at > CURRENT_TIMESTAMP RETURNING data"

// Finish takes what was started under an id, once.
func Finish(db cloud.DB, id string) (string, error) {
	row, err := cloud.One[FinishRow](db, qFinish, id)
	return row.Data, err
}

const qStartedLike = "SELECT id, data, expires_at FROM ceremonies WHERE id LIKE ? AND expires_at > CURRENT_TIMESTAMP ORDER BY expires_at"

// StartedLike is everything started whose id is like a pattern, the first to expire first: the invitations.
func StartedLike(db cloud.DB, id string) ([]CeremoniesRow, error) {
	return cloud.Query[CeremoniesRow](db, qStartedLike, id)
}
