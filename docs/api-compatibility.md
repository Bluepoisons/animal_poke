# API N-1 Compatibility Policy

AP-089 defines the compatibility, deprecation, and sunset lifecycle for the Animal Poke API. This document is the primary reference for breaking changes, deprecation windows, and client migration.

## Versioning scheme

The API uses URL-prefix versioning: `/api/v1`. A new major version (`/api/v2`) starts when a breaking change cannot be introduced through additive-only evolution. Patches and minor additions happen within the same prefix.

## Client identity

Every client MUST send `X-Client-Version: <semver>` on every request. The server uses this header for:
- Deprecation/Sunset response headers
- Capability negotiation
- N-1 compatibility gate enforcement

If the header is absent, the server treats the client as `unknown` and applies the most conservative compatibility rules.

## Backward-compatible changes (always allowed)

These changes MAY be deployed without a deprecation cycle:
- Adding a new optional field to a JSON response body
- Adding a new optional query parameter with a default that preserves current behavior
- Adding a new endpoint
- Adding a new enum value that old clients treat as unrecognized (not as an error)
- Relaxing a validation constraint (e.g. increasing max length)
- Adding a new response header

## Deprecation (minimum 2 release cycles)

When a field, parameter, or endpoint must be removed:

1. **Annotate** the OpenAPI schema with `deprecated: true` and `x-sunset-date`.
2. **Register** the deprecation in `DeprecatedOperations` in the server config.
3. **Emit** `Deprecation: true` and `Sunset: <RFC 1123 date>` response headers.
4. The sunset date MUST be at least 2 production releases (typically 4 weeks) from first deprecation.
5. The deprecated path continues to function correctly throughout the window.
6. Old clients receive a `Warning: 299 - "…"` header explaining the migration path.

## Breaking changes (require a new major version)

These changes MUST NOT be deployed within `/api/v1`:
- Removing a field from a response body
- Removing an endpoint
- Changing a field type
- Changing authentication requirements
- Changing idempotency semantics
- Reducing the support window for old cursor formats or offline-queued payloads

Breaking changes require `/api/v2` and a documented migration plan.

## N-1 contract gate

CI runs a contract matrix test that verifies:

1. The current OpenAPI spec has no unreported breaking diffs against the previous release.
2. Every `deprecated: true` annotation has a corresponding `x-sunset-date` in the future or a just-expired date (≤7 days past).
3. Every registered deprecated operation emits `Deprecation` and `Sunset` headers.
4. Old-cursor, old-idempotency-payload, and old-offline-request fixtures still produce valid results within the N-1 window.

## Offline client support

- Offline-captured animals queued in IDB MUST be accepted by the server for at least 2 versions after capture.
- IDB schema version bumps MUST include a migration path; the server MUST accept payloads from the previous schema for 2 release cycles.
- Tombstone markers from deleted accounts MUST be honored for the full sunset window.

## Capability negotiation

Clients MAY send `X-Client-Capabilities: <csv>` to declare optional features. The server responds with `X-Server-Capabilities: <csv>` listing what is actually enabled.

The minimum client version is configured via `MIN_CLIENT_VERSION` and emitted in `X-Min-Client-Version` on every response. Clients below the minimum receive a 426 Upgrade Required response.
