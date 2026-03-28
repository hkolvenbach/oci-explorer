# ADR-002: Registry Authentication via OIDC

- **Status:** Proposed
- **Date:** 2026-03-27

## Context

The publicly hosted OCI Explorer instance can only access public container images. Users cannot inspect or scan private repositories. Additionally, all users share a single outbound IP address, which means Docker Hub's anonymous rate limit (100 pulls per 6 hours) is consumed collectively -- a few active users can exhaust the quota for everyone.

Authenticated Docker Hub users get 200 pulls per 6 hours (or unlimited on paid plans). Providing per-user authentication would both unlock private repositories and give each user their own rate limit allocation.

## Decision Drivers

- Enable private repository access on the public instance
- Per-user rate limits to prevent shared quota exhaustion
- Security of credential handling (tokens must be short-lived, scoped, encrypted)
- UX simplicity (users shouldn't need to understand OAuth flows)
- Compatibility with the response cache (ADR-001)

## Considered Approaches

### Per-Registry OAuth2 Flows

Users authenticate directly with their container registry (Docker Hub, GHCR, GCR, ECR) via browser-based OAuth2. The server receives a scoped access token and uses it for registry operations on behalf of the user.

**Pros:** Standard protocol, per-user rate limits, registry-native scoping.
**Cons:** Each registry has different OAuth2 endpoints, scopes, and token formats. Docker Hub uses OAuth2 with `registry.docker.io` as the token endpoint. GHCR uses GitHub OAuth Apps. GCR/Artifact Registry uses Google OAuth2. ECR uses AWS STS. Implementing and maintaining per-provider flows is significant effort.

### Docker Credential Forwarding

Users paste a Docker Hub access token (or personal access token) directly into the UI. The server uses this token for registry operations.

**Pros:** Simple to implement, no OAuth flow needed.
**Cons:** Less secure (tokens may have broad scope), no automatic refresh, poor UX (users must manually generate and paste tokens), doesn't work well for registries that use short-lived tokens.

### OIDC Proxy Pattern

A single OIDC identity provider (e.g., Auth0, Clerk, or self-hosted) handles application login. Users then link their registry credentials in a profile/settings page. The app stores encrypted credentials per user and uses them for registry operations.

**Pros:** Best UX (single login, credential linking), cleanest separation of concerns.
**Cons:** Highest implementation cost (needs user database, credential storage, session management, OIDC integration). Introduces a user account system where none exists today.

## Cache Interaction

The response cache (ADR-001) keys entries by SHA256 digest. For public images, the same digest always returns the same content regardless of who fetches it -- caching is straightforward.

For private images, access control is needed on cache reads. Options:

1. **Skip caching for authenticated requests**: simplest, but loses caching benefits for private images.
2. **Scope cache keys**: include a visibility flag or namespace (e.g., `inspect/private/{user}/sha256:...`). Adds complexity and reduces cache hit rates.
3. **Cache with ACL check**: store results with the repository's visibility, check the requesting user's access before serving. Requires tracking which users can access which repositories.

The recommended approach for an initial implementation is option 1 (skip caching for authenticated/private requests). Most cache value comes from popular public images anyway. Private image caching can be added later if demand warrants it.

## Security Considerations

- **Token storage**: tokens must be encrypted at rest and short-lived. Prefer tokens scoped to `repository:read` only.
- **Session management**: use secure, httpOnly, SameSite cookies. Sessions should expire after a reasonable period (e.g., 24 hours).
- **Scope minimization**: request the minimum OAuth2 scope needed (read-only registry access). Never request write access.
- **Token refresh**: implement token refresh flows where supported to avoid requiring users to re-authenticate frequently.
- **Audit logging**: log which registries are accessed per session (without logging the tokens themselves).

## Recommendation

Implement after the response cache (ADR-001) is stable. The two features are complementary:

- **Cache** reduces total registry calls (benefits all users, especially for popular public images)
- **Auth** provides per-user rate limits and private repository access

Starting with per-registry OAuth2 for Docker Hub and GHCR (the two most common registries) would cover the majority of use cases. Other registries can be added incrementally.
