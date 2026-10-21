Swagger UI assets are vendored from `swagger-ui-dist@5.33.2` (Apache-2.0).
Refresh with `sh scripts/update_swagger.sh` from the repository root.
`init.js` is repository-owned and loads only the same-origin `/openapi.json`.
No external CDN or persisted authorization is used.
