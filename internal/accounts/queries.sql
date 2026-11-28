-- The statements charter's own sign-in and admin code runs on the tables of schema.sql (package api: people.go,
-- admin.go), in the form an app's db/queries.sql has. queries.gen.go is written from them by the generator that
-- writes an app's (internal/dbcode, with sqlc): UPDATE=1 go test ./internal/accounts. charter check runs them
-- against the schema of an app that has the sign-in screen, so a table changed there is said before it is deployed.

-- name: PersonByToken :one
-- PersonByToken is whoever a token's hash is, with what the token is and may do, while it has not expired.
SELECT users.id, users.name, users.role, tokens.scopes, tokens.kind FROM tokens JOIN users ON users.id = tokens.user_id WHERE tokens.hash = ? AND (tokens.expires_at IS NULL OR tokens.expires_at > CURRENT_TIMESTAMP);

-- name: Person :one
-- Person is one person, by their id.
SELECT id, name, role FROM users WHERE id = ?;

-- name: CreatePerson :exec
-- CreatePerson keeps a new person.
INSERT INTO users (id, name, role) VALUES (?, ?, ?);

-- name: Everyone :many
-- Everyone is the app's people, the first to sign up first.
SELECT id, name, role FROM users ORDER BY created_at, id LIMIT 500;

-- name: SetRole :execrows
-- SetRole makes a person what role says, and says whether there was such a person.
UPDATE users SET role = ? WHERE id = ?;

-- name: RemoveTokensOf :exec
-- RemoveTokensOf takes away every session and token someone holds.
DELETE FROM tokens WHERE user_id = ?;

-- name: RemovePasskeysOf :exec
-- RemovePasskeysOf takes away every passkey someone signs in with.
DELETE FROM passkeys WHERE user_id = ?;

-- name: RemovePerson :exec
-- RemovePerson takes a person away: after their tokens and passkeys, in one batch.
DELETE FROM users WHERE id = ?;

-- name: Passkeys :many
-- Passkeys is the passkeys someone signs in with.
SELECT credential FROM passkeys WHERE user_id = ?;

-- name: AddPasskey :exec
-- AddPasskey keeps a passkey, or what it is now when it is kept already.
INSERT INTO passkeys (id, user_id, credential) VALUES (?, ?, ?) ON CONFLICT (id) DO UPDATE SET credential = excluded.credential;

-- name: IssueToken :exec
-- IssueToken keeps a session or a token by its hash, never itself.
INSERT INTO tokens (hash, user_id, kind, name, scopes, expires_at) VALUES (?, ?, ?, ?, ?, ?);

-- name: Tokens :many
-- Tokens is the tokens of one kind someone holds, newest first.
SELECT id, name, scopes, created_at FROM tokens WHERE user_id = ? AND kind = ? ORDER BY id DESC;

-- name: RevokeToken :exec
-- RevokeToken takes away one of someone's tokens, by its id or by its hash.
DELETE FROM tokens WHERE user_id = ? AND (id = ? OR hash = ?);

-- name: ForgetStarted :exec
-- ForgetStarted takes away what was started and never finished.
DELETE FROM ceremonies WHERE expires_at < CURRENT_TIMESTAMP;

-- name: Start :exec
-- Start keeps something started (a sign-in, an invitation) until it is finished or expires.
INSERT INTO ceremonies (id, data, expires_at) VALUES (?, ?, ?);

-- name: Started :one
-- Started is what was started under an id, while it has not expired.
SELECT data FROM ceremonies WHERE id = ? AND expires_at > CURRENT_TIMESTAMP;

-- name: Finish :one
-- Finish takes what was started under an id, once.
DELETE FROM ceremonies WHERE id = ? AND expires_at > CURRENT_TIMESTAMP RETURNING data;

-- name: StartedLike :many
-- StartedLike is everything started whose id is like a pattern, the first to expire first: the invitations.
SELECT id, data, expires_at FROM ceremonies WHERE id LIKE ? AND expires_at > CURRENT_TIMESTAMP ORDER BY expires_at;
