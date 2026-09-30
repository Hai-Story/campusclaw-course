# Tasks

## 1. Skeleton and configuration

- [x] 1.1 Create Go and React/Vite directory skeletons.
- [x] 1.2 Load and validate all required configuration from environment variables.
- [x] 1.3 Add `.env.example`, `.gitignore` and `.dockerignore` without real secrets.

## 2. Data and seeds

- [x] 2.1 Create the five tables with non-null, indexed tenant columns.
- [x] 2.2 Add idempotent A/B class users, materials and knowledge entries.
- [x] 2.3 Hash all seed passwords with bcrypt.

## 3. Authentication

- [x] 3.1 Implement login, signed Bearer tokens backed by revocable server sessions, `/api/me` and logout.
- [x] 3.2 Issue a fresh token on login and immediately revoke its server session on logout; reject old cookie-only requests.
- [x] 3.3 Add uniform credential failures and username+IP login throttling.

## 4. Isolation and authorization

- [x] 4.1 Filter list/search by the session class only.
- [x] 4.2 Fetch objects by ID then enforce class ownership with uniform 404 responses.
- [x] 4.3 Enforce teacher-only upload on the server.

## 5. Upload and ingestion

- [x] 5.1 Validate extension, request/file size, UTF-8 and non-empty content.
- [x] 5.2 Generate storage names server-side and prevent static upload exposure.
- [x] 5.3 Write material and knowledge rows in one transaction and clean files on failure.
- [x] 5.4 Implement authenticated details and download.

## 6. Frontend

- [x] 6.1 Store the Bearer token in per-tab `sessionStorage`, restore identity from `/api/me` and redirect on 401.
- [x] 6.2 Implement login, logout, material list, detail and authenticated download.
- [x] 6.3 Gate the upload UI on `/api/me` and provide XHR progress.
- [x] 6.4 Add PKU red brand styling, light/dark themes, list/grid views and responsive layout.
- [x] 6.5 Add class-scoped search, Ctrl/Cmd+K command palette and toast feedback.
- [x] 6.6 Render Markdown with GFM without enabling raw HTML.

## 7. Compose and documentation

- [x] 7.1 Add web/api/db services with only web exposed.
- [x] 7.2 Persist database and uploads; keep uploads mounted only on api.
- [x] 7.3 Add readiness retry and liveness-only `/health`.
- [x] 7.4 Document reproducible startup, accounts, scope, non-goals, 404 policy and single-instance boundary.

## 8. Verify and archive

- [x] 8.1 Run backend tests and frontend production build.
- [x] 8.2 Run strict OpenSpec validation.
- [ ] 8.3 Verify unauthenticated, student-upload and cross-class negative paths.
- [ ] 8.4 Record evidence and archive the completed change.

## 9. Teacher material deletion

- [x] 9.1 Add a repeatable seed-deletion tombstone migration and skip tombstoned seed materials during startup; verify a deleted seed is not restored.
- [x] 9.2 Implement teacher-only same-class `DELETE /api/materials/{id}` with uniform cross-class/unknown 404, processing-job conflict, cascading database removal, vector removal and private-file cleanup; verify student and failure paths.
- [x] 9.3 Add confirmed delete actions in the teacher material UI and refresh visible materials after success; verify students cannot see delete controls.
- [x] 9.4 Verify deletion of an uploaded and a seed material against Compose, including search and ask source disappearance after restart; update README and run strict OpenSpec validation.
