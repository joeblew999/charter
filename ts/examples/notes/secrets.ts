// The Worker's secrets, by name: one list, which cloudflare.config.ts and the scripts import.

/** A deploy needs every one. */
export const secrets = ["READ_TOKEN", "WRITE_TOKEN"] as const;

/** Set where the environment that deploys or runs the Worker has them: who else it trusts (Cloudflare
 * Access, an OpenID Connect issuer). Unset, the Worker trusts no one that way. */
export const optionalSecrets = ["ACCESS_TEAM_DOMAIN", "ACCESS_AUD", "OIDC_ISSUER", "OIDC_AUDIENCE"] as const;
