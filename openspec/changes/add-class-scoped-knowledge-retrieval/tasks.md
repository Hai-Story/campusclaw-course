## 1. Data model and configuration

- [ ] 1.1 Add repeatable MySQL migrations for `knowledge_chunks` and `knowledge_index_jobs`, with class-scoped indexes and unique versioned job/fragment IDs; verify a second migration run leaves data and row counts unchanged.
- [ ] 1.2 Add validated retrieval configuration for local embedding model/version, vector dimension, Qdrant endpoint, bounded query limits and retry policy; verify missing or mismatched required values fail startup clearly.
- [ ] 1.3 Add internal-only Qdrant and local embedding services with persistent vector/model volumes to Compose; verify `docker compose config` shows only `web` publishing a host port.

## 2. Index creation and recovery

- [ ] 2.1 Implement Unicode-codepoint passage splitting with stable IDs, source offsets and text hashes; verify Chinese/English, Markdown, empty text, overlap and exact substring reconstruction in unit tests.
- [ ] 2.2 Insert a versioned pending index job in the existing teacher-upload transaction while keeping its 201 response; verify successful upload creates one job and a rolled-back upload leaves no job, material, knowledge row or file.
- [ ] 2.3 Implement idempotent backfill of current-version jobs for all existing knowledge entries; verify repeated backfill creates no duplicates and changes no material IDs or source bodies.
- [ ] 2.4 Implement worker leasing, local embedding, MySQL chunk upsert, Qdrant point upsert, retry/backoff and ready/failed transitions; verify restart after vector upsert produces one result per stable chunk ID.
- [ ] 2.5 Add an operator-triggered retry/rebuild path for failed jobs or index-version changes; verify a missing vector collection can be rebuilt from MySQL without re-uploading materials.

## 3. Authorized retrieval API

- [ ] 3.1 Add authenticated `GET /api/knowledge/search` with trimmed 1–200-codepoint `q`, default 10/max 20 `limit`, and 400/401 responses; verify boundary inputs and unauthenticated requests in handler tests.
- [ ] 3.2 Query Qdrant with server-session class and active index version, then hydrate candidates from class-filtered MySQL rows and extract snippets from the stored body; verify forged `class_id`, stale point, missing source and cross-class candidates never leak titles, excerpts or IDs.
- [ ] 3.3 Expose `ready/building/degraded` index state from current-class jobs and return 503 for embedding/vector outages; verify no-match 200, pending indexing, failed indexing and dependency outage are distinguishable.
- [ ] 3.4 Return stable source IDs, title, original filename and `[start,end)` codepoint offsets without storage paths or vectors; verify every returned excerpt equals the cited substring in the authenticated material detail.

## 4. Web search and source navigation

- [ ] 4.1 Add a separate knowledge-search mode without replacing `/api/materials?q=...`; verify material-list search and command palette still work as before.
- [ ] 4.2 Render result excerpts, source metadata, empty/pending/degraded/error states and bounded input; verify Markdown/HTML from an uploaded excerpt never executes in the results UI.
- [ ] 4.3 Open the existing authenticated material detail from a result, highlight the cited codepoint range and retain download; verify stale or cross-class source navigation shows the same unavailable state as an unknown source.
- [ ] 4.4 Cancel or ignore out-of-order search responses when the query changes; verify a slow earlier request cannot replace the current query's results.

## 5. Integration and documentation

- [ ] 5.1 Extend A/B class integration fixtures and checks for same-class semantic match, forged class override, cross-class/unknown source 404 equality, unauthenticated 401 and safe snippets; verify the end-to-end script passes against Compose.
- [ ] 5.2 Verify existing seed materials and a fresh teacher upload become searchable after backfill/worker processing; interrupt indexing and restart services to verify recovery without duplicate results.
- [ ] 5.3 Document model/version pinning, resource needs, index states, rebuild procedure, rollback and the unchanged material-search contract in README; verify the documented commands work on a clean Compose volume.
- [ ] 5.4 Run Go tests/vet/build, frontend production build and `openspec validate add-class-scoped-knowledge-retrieval --strict`; record actual outcomes and any unavailable environment checks before marking implementation complete.
