// The Worker's secrets, by name: one list, which cloudflare.config.ts and charter.config.ts import.

/** A deploy needs every one: the API's own tokens (api/handlers.ts: trusted). */
export const secrets = ["READ_TOKEN", "WRITE_TOKEN"] as const;

/** Set where the environment that deploys or runs the Worker has them: who else it trusts (Cloudflare
 * Access, an OpenID Connect issuer; unset, no one that way), and the secret that signs its webhook
 * (unset: "whsec", for trying it out). */
export const optionalSecrets = ["ACCESS_TEAM_DOMAIN", "ACCESS_AUD", "OIDC_ISSUER", "OIDC_AUDIENCE", "WEBHOOK_SECRET"] as const;
