# Custom RAG (Go) — Design Specification

> **Status:** Draft for implementation planning. Corpus: RFC 9000 (QUIC) + [quinn-rs/quinn](https://github.com/quinn-rs/quinn).

## Goals

- Lightweight, maintainable **Go** service layout with **Gin** for HTTP.
- **Split ingest and query** deployables so each tier scales independently.
- **Vector search** via **Qdrant** (default); `VectorStore` interface allows a future **pgvector** adapter without rewriting callers.
- **RAG:** OpenAI embeddings in v1; **chat** switchable between **OpenAI** and **Gemini** via configuration.
- **12-factor** configuration, structured logging, stateless query workers.
- **SOLID-style boundaries:** small interfaces (`Chunker`, `Embedder`, `VectorStore`, `Retriever`, `Generator`), Gin handlers stay thin.
- **Authentication / authorization:** **JWT** (Bearer) on **both** **query** and **ingest** HTTP APIs, each acting as an OIDC **resource server**; **Keycloak** as the initial OIDC provider with a small user set (2–3 accounts) to start. First rollout uses **confidential backend-only** clients; target topology is **public SPA + BFF** (see phased identity section and **B-6**).
- **Query performance:** an **internal cache** inside the query service to avoid redundant embedding (and optionally retrieval / LLM) work.

## Non-goals (v1)

- Microsoft **Entra ID** as primary IdP or full org-wide **SSO** (see **Backlog B-4**).
- Fine-grained ABAC beyond simple scopes/roles carried in JWT claims.
- Automatic migration between embedding models without an explicit re-index job.

## Architecture

### Deployables

| Binary / image | Responsibility |
|----------------|----------------|
| **ingest** | Fetch/normalize sources (RFC text, Quinn tree), chunk, **embed (OpenAI)**, upsert Qdrant with rich payload metadata. **Auth:** every HTTP request requires `Authorization: Bearer <JWT>` validated the same way as query (JWKS, `iss`, `aud`, lifetime). Callers hold a **confidential** Keycloak client and use **client credentials** (service account) with scope **`rag:ingest`** for automation, or a **user access token** that includes `rag:ingest` if a human triggers ingest from a trusted tool. |
| **query** | Gin API: **JWT validation** (OIDC), embed user query (OpenAI), retrieve from Qdrant, optional **internal cache**, assemble context, call configured **chat** provider (OpenAI or Gemini), return answer + citations. Requires scope **`rag:query`** (or equivalent mapped claim). |

Shared code lives in a single Go module (e.g. `internal/domain`, `internal/qdrantstore`, `internal/llm/...`) consumed by both commands.

### Data flow

1. **Ingest:** `Authorization: Bearer <JWT>` → validate claims (including **`rag:ingest`**) → source → chunks with payload `{ source, uri, section_or_path, content_sha256, text, ... }` → vector → Qdrant upsert.
2. **Query:** `Authorization: Bearer <JWT>` → validate claims → question → **cache lookup** (see below) → **OpenAI embedding** (on miss) → Qdrant similarity search (+ optional payload filters, e.g. `source=rfc9000`) → ranked chunks → LLM prompt with citations → response (optional **write-through** cache for stable layers).

### Technology choices (v1)

- **Go:** Toolchain per team standard (e.g. 1.26.x); `go` directive in `go.mod` matches CI and local builds.
- **HTTP:** [Gin v1.12.0](https://pkg.go.dev/github.com/gin-gonic/gin@v1.12.0) (module requires Go ≥ 1.25 per Gin metadata).
- **Vector DB:** [qdrant/go-client](https://github.com/qdrant/go-client); collection vector size must match the embedding model dimension.
- **Embeddings (v1):** [openai/openai-go](https://github.com/openai/openai-go) — `Embedder` interface; **only OpenAI implementation** shipped initially.
- **Chat:** Pluggable `Generator` — **OpenAI** and **Gemini** ([googleapis/go-genai](https://github.com/googleapis/go-genai), `google.golang.org/genai`) behind config.
- **Auth:** JWT access tokens issued by **Keycloak** (OIDC); **query** and **ingest** validate signature via **JWKS** from issuer **well-known** metadata, enforce **issuer** (`iss`) and **audience** (`aud`) / **authorized party** (`azp`) per configured policy, and check **expiry** / `nbf`. Authorization uses **client scopes** (preferred) or **realm roles** mapped to **`rag:query`** and **`rag:ingest`**.

### Authentication and authorization

**Model:** Both **query** and **ingest** HTTP servers are **resource servers**. Every mutating or sensitive call includes `Authorization: Bearer <access_token>`. Tokens are issued by **Keycloak**; validation logic SHOULD be shared code (e.g. `internal/auth/oidc.go`) with per-route **required scope** checks.

#### End-to-end flow (Keycloak realm user → auth code → token → APIs)

This is the **overall operator experience** the system is designed for:

1. **Load user in realm:** An administrator configures the **realm**, **confidential** OAuth clients, **client scopes** (`rag:query`, `rag:ingest`), redirect URIs, and **creates users** (credentials, optional groups / role mappings).
2. **Authenticate against Keycloak (OIDC):** The user opens a browser and starts the standard **Authorization Code** flow against Keycloak’s **authorization** endpoint—`response_type=code`, correct **`client_id`**, **`redirect_uri`**, **`scope`** (include `openid` and the API scopes you need, e.g. `rag:query`), and **`state`**.
3. **Receive authorization code:** After successful login, Keycloak **redirects** to **`redirect_uri`** with a short-lived **`code`** (and echoes **`state`**). Phase 1 uses the repo **`cmd/oauth-dev`** helper (`go run ./cmd/oauth-dev`) serving **`http://127.0.0.1:5555/oauth-callback`** — it prints the query string to the terminal and shows it in the browser for copy-paste (see [Keycloak plan](../plans/2026-04-15-keycloak-realm-and-jwt.md) Part 0).
4. **Generate tokens from the code:** A **trusted local tool** (e.g. CLI with the **confidential** client **secret**) calls Keycloak’s **token** endpoint with `grant_type=authorization_code`, the **`code`**, **`redirect_uri`**, and client authentication, and receives **`access_token`** (JWT) and optionally **`refresh_token`**. **Never** put the client secret in browser-only static assets.
5. **Call query / ingest:** HTTP clients send `Authorization: Bearer <access_token>`. **Query** requires scope **`rag:query`**; **ingest** requires **`rag:ingest`**. The Go services validate the JWT (JWKS, `iss`, `aud`/`azp`, lifetime) and enforce scopes.

**Ingest automation (typical):** CI or a job runner uses the **ingest** confidential client with **`client_credentials`** to obtain a service-account access token that includes **`rag:ingest`**, without a human authorization code. Human-triggered ingest is still supported if the user’s token carries **`rag:ingest`**.

#### Keycloak topology (phased)

| Phase | Who signs in | Clients | Notes |
|-------|----------------|---------|--------|
| **Phase 1 (now)** | **2–3 pilot users** (realm users) | **Confidential, backend-only** Keycloak clients (no public SPA in the critical path). | **Human token UX (decision):** pilots use **Keycloak Account Console** (and related realm login) with the standard **Authorization Code** flow in the browser—**no password (ROPC) grant**. Configure a **confidential** query client with **Valid redirect URIs** **`http://127.0.0.1:5555/oauth-callback`** (and optionally `http://localhost:5555/oauth-callback`), matching **`go run ./cmd/oauth-dev`**. The helper prints **`code`** / **`state`** to the terminal and browser so the pilot can **paste them into a small local CLI** that performs the **token exchange** using the **client secret**—**never** put the client secret in browser-only assets. Callers then pass the access token as `Authorization: Bearer` to **query**. **Ingest** automation continues to use a **second confidential client** with **client credentials** and **`rag:ingest`**. |
| **Phase 2 (target)** | End users in browser | **Public SPA** + **BFF** (confidential server-side OAuth client). | SPA uses **Authorization Code + PKCE**; BFF holds the **client secret**, exchanges code for tokens, attaches `Authorization` when calling query/ingest. See **B-6**. |

**Realm and users:**

- Dedicated **realm** (or env-specific realms) with **2–3 users** for Phase 1.
- **Standard OIDC discovery** (`/.well-known/openid-configuration`) for JWKS; cache keys and handle rotation.

**Scopes (v1 contract):**

- **`rag:query`** — required on query RAG endpoints.
- **`rag:ingest`** — required on ingest HTTP endpoints.

**Separation of privilege:** End-user tokens used for query SHOULD NOT include `rag:ingest` unless a break-glass operator role is explicitly assigned. Ingest runners use the **ingest** confidential client and service account.

**Go service responsibilities:**

- Shared middleware: parse Bearer token, **validate JWT**, attach `sub`, `azp`, and resolved scopes to `context.Context`.
- Reject missing/invalid tokens with `401`; missing required scope with `403`.
- Log **`sub`**, **`azp`**, client id (if distinct), and request id; never log raw JWT.

**JWT `aud` / `azp`:** Configure Keycloak **audience** mappers (or resource-access claims) so access tokens intended for these APIs carry a predictable **`aud`** (e.g. resource-server client id or custom audience) **or** enforce **`azp`** against an allowlist of confidential client ids. Exact mapper layout depends on Keycloak version—document the chosen pattern in the implementation plan and keep **dev/stage/prod** aligned.

**Future (see B-4):** **Microsoft Entra ID** (Azure AD) as IdP with **SSO**—Keycloak broker vs native Entra JWKS; may interact with Phase 2 BFF flows.

### Internal query cache (query service)

**Purpose:** Reduce latency and OpenAI embedding usage when the same or equivalent questions repeat (common in demos, retries, and thin clients).

**Layers (recommended order of implementation):**

1. **Embedding cache (required in v1 spec):** Key = hash of **canonicalized** question string + **embedding model id** + relevant retrieval defaults if they affect embedding-only path (normally omit). Value = dense **embedding vector**. TTL configurable (e.g. 5–60 minutes); bounded size (e.g. in-process **LRU** max entries). **Always include** `sub` (JWT subject) in the cache key if there is any chance of per-user retrieval filters in the future; for purely global retrieval, document whether `sub` is included—default **include `sub`** for safer multi-tenant evolution.
2. **Retrieval cache (optional):** Key = embedding hash or question hash + Qdrant filter fingerprint + `top_k`. Value = serialized chunk ids + scores + payload pointers. Shorter TTL than embeddings if corpus changes often.
3. **Full response cache (optional, conservative):** Key = question hash + chat model + provider + prompt version. **Short TTL** only; higher staleness risk when corpus is re-ingested.

**Implementation notes:**

- **Process-local** cache is sufficient for first deployments (single replica or low replica count). For **horizontal scale** of query replicas, move to **Redis** or similar shared cache with the same key schema (**Backlog** item if needed).
- Invalidate or shorten TTL on **ingest completion** events if retrieval/response caching is enabled (event hook or manual ops in early phases).
- Expose metrics: hit/miss rate, eviction count, approximate memory.

### Configuration (12-factor)

Illustrative environment variables (final names to be normalized in implementation):

| Variable | Purpose |
|----------|---------|
| `OPENAI_API_KEY` | OpenAI API (embeddings + optional chat). |
| `OPENAI_EMBEDDING_MODEL` | Embedding model id (default e.g. `text-embedding-3-small`). |
| `LLM_CHAT_PROVIDER` | `openai` \| `gemini`. |
| `OPENAI_CHAT_MODEL` | Chat model when provider is OpenAI. |
| `GOOGLE_API_KEY` / `GEMINI_API_KEY` | Gemini Developer API key when chat provider is Gemini. |
| `GEMINI_CHAT_MODEL` | e.g. `gemini-2.5-flash`. |
| `QDRANT_HOST`, `QDRANT_PORT` (or URL) | Qdrant connectivity for ingest and query. |
| `GIN_MODE` | `release` in production. |
| `PORT` | HTTP listen port for query service. |
| `OIDC_ISSUER_URL` | Keycloak realm issuer base URL used for discovery (e.g. `https://keycloak.example/realms/myrealm`). |
| `OIDC_AUDIENCE` | Expected JWT `aud` (resource server audience / client id—align with Keycloak **Audience** mapper or `azp` policy as designed). |
| `OIDC_SKIP_TLS_VERIFY` | **Dev only** — never `true` in production. |
| `CACHE_EMBED_MAX_ENTRIES` | LRU cap for embedding cache (e.g. `5000`). |
| `CACHE_EMBED_TTL` | Embedding cache entry TTL (e.g. `10m`). |
| `CACHE_RETRIEVAL_ENABLED` | `true` / `false` for optional retrieval cache. |
| `CACHE_RESPONSE_ENABLED` | `true` / `false` for optional full-response cache (default `false`). |

Secrets only via env or secret store; never committed.

### Interfaces (extensibility)

- **`Embedder`:** `Embed(ctx, texts []string) ([][]float32, error)` (or equivalent types). v1: `OpenAIEmbedder` only; see **Backlog** for additional providers.
- **`Generator`:** `Generate(ctx, system, user string, opts ...) (string, error)` (shape to be refined). Implementations: OpenAI chat, Gemini `GenerateContent`.
- **`VectorStore`:** create/upsert/query abstractions over Qdrant.

Changing **embedding model** requires **collection compatibility** (dimension) and a **re-index**; not a runtime hot-swap.

---

## Backlog

### B-1: Both stages provider-switchable (embeddings + chat)

**Intent:** Experiment with non-OpenAI embedding providers (e.g. Gemini / Vertex embeddings) while keeping the same ingest/query split.

**Scope:**

- Add config such as `EMBEDDING_PROVIDER=openai|gemini|...` (exact enum TBD).
- Implement additional **`Embedder`** packages mirroring the chat provider pattern.
- Startup validation: allowed `(embedding_provider, embedding_model)` and `(chat_provider, chat_model)` pairs; fail fast on impossible combinations.
- **Re-index playbook** when switching embedding provider or model:
  - Content-addressed chunk ids / hashes for idempotency.
  - Prefer **new Qdrant collection** (or snapshot + blue/green) when vector dimension or distance behavior changes.
  - Document cutover: dual-write optional, read flag for collection name, rollback.

**Dependencies:** Provider SDK support, pricing/latency evaluation, dimension mapping to Qdrant `VectorParams.Size`.

### B-2: pgvector `VectorStore` implementation

Optional second backend for teams constrained to Postgres-only operations; same `VectorStore` interface.

### B-3: Repo Agent Skills

Skills under `.cursor/skills/` (or project convention) for: Go RAG package boundaries, 12-factor env catalog, scalable ingest (batching, rate limits, idempotency).

### B-4: Microsoft Entra ID + SSO

**Intent:** Organization-wide sign-in via **Entra ID** (Azure AD), still presenting **JWT** access tokens to this resource server.

**Approaches (pick during implementation of B-4):**

- **Keycloak Identity Brokering:** Entra as upstream IdP; tokens presented to the API remain issued by Keycloak (simpler single JWKS for the app).
- **Native Entra:** Resource server trusts **Entra issuer** and **JWKS**; Keycloak decommissioned or retained only for non-Entra users; may require **multi-issuer** support in middleware.

**Also consider:** Conditional Access, token lifetime, **on-behalf-of** flows if a BFF or multi-tier architecture appears later.

### B-5: Distributed query cache

When query replicas > 1 and cache hit rate matters across instances, replace or supplement in-process LRU with **Redis** (or equivalent) using the same key taxonomy; keep embedding cache semantics identical.

### B-6: Public SPA + BFF (target Keycloak topology)

**Intent:** Replace Phase 1 “backend-only” token acquisition with a **browser SPA** (public client, PKCE) and a **BFF** that holds the **confidential** OAuth client secret, performs token exchange / refresh, and forwards `Authorization: Bearer` to **query** (and optionally **ingest** if exposed only to operators via BFF).

**Scope:** Harden CORS, cookie/session strategy for BFF, refresh rotation, CSRF policy for BFF routes, and Keycloak client config (redirect URIs, web origins).

---

## Open questions (resolve before coding)

- Exact Keycloak **mapper** configuration for **`aud`** vs **`azp`** checks (document per environment once Keycloak version is pinned).

---

## References

- RFC 9000: https://www.rfc-editor.org/rfc/rfc9000.html  
- Quinn: https://github.com/quinn-rs/quinn  
- Gin: https://pkg.go.dev/github.com/gin-gonic/gin@v1.12.0  
- Qdrant Go client: https://github.com/qdrant/go-client  
- OpenAI Go: https://github.com/openai/openai-go  
- Google Gen AI Go: https://github.com/googleapis/go-genai  
- Keycloak (OIDC): https://www.keycloak.org/documentation  
- Microsoft Entra ID (OIDC): https://learn.microsoft.com/en-us/entra/identity-platform/  
