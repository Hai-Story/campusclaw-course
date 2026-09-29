## Purpose

让已登录的教师和学生从自己班级的知识库正文中找到相关片段，并凭可核对的来源信息回到原始材料；检索及引用始终遵守现有的班级隔离边界。

## ADDED Requirements

### Requirement: Class-scoped knowledge search

The system MUST provide a separate knowledge-search API for authenticated teachers and students. It MUST derive the search class from the server-side session, return only content belonging to that class, and never accept a client-supplied class as authority.

#### Scenario: Same-class retrieval

- **WHEN** a logged-in class A user searches for a phrase represented in a class A knowledge entry
- **THEN** the response returns relevant class A excerpts with their source references
- **AND** no class B title, excerpt, identifier, or source metadata appears

#### Scenario: Client class override is ignored

- **WHEN** a class A user sends a search request with class B's `class_id` in the query or headers
- **THEN** the server searches only class A and does not reveal whether class B has matching content

#### Scenario: Unauthenticated search

- **WHEN** a caller without a valid session invokes the knowledge-search API
- **THEN** the server returns 401 without any knowledge or source data

### Requirement: Bounded search contract

The system MUST accept a non-empty UTF-8 search query, impose a documented length limit and bounded result count, and distinguish an empty match set from a failed search.

#### Scenario: Valid search has no matches

- **WHEN** a valid query has no matching indexed passage in the user's class and indexing is complete
- **THEN** the server returns 200 with an empty results list and a ready index state

#### Scenario: Invalid query

- **WHEN** the query is blank after trimming or exceeds the documented length limit
- **THEN** the server returns 400 without executing a search

#### Scenario: Search dependency unavailable

- **WHEN** the retrieval dependency cannot serve a valid search
- **THEN** the server returns 503 with a generic retryable error and no passages from another class

### Requirement: Verifiable source references

Every returned passage MUST contain an excerpt taken from the stored source body, the material ID, material title, original filename, and stable start/end offsets into that body. The web app MUST let the user open the authorized material detail at the cited passage and inspect its surrounding text.

#### Scenario: Follow a result to its source

- **WHEN** a user selects a search result
- **THEN** the app opens the corresponding authenticated material detail and identifies the cited passage in the original body
- **AND** the user can access the existing authenticated download action

#### Scenario: Stale or cross-class source reference

- **WHEN** a user requests a citation whose source material is absent or belongs to another class
- **THEN** the source access returns the same 404 status and response body as an unknown material or citation
- **AND** the response reveals no source text or metadata

### Requirement: Searchable ingestion and recovery

The system MUST index existing knowledge entries and new successful uploads, preserve their source mapping, and recover indexing after interruption without duplicating search results. A successfully stored upload MUST remain stored if indexing is delayed; users MUST be told when their class's index is still catching up.

#### Scenario: Existing entries become searchable

- **WHEN** retrieval is enabled for an installation with existing seed or uploaded knowledge entries
- **THEN** a backfill indexes them without changing their material IDs or source bodies

#### Scenario: Upload waits for indexing

- **WHEN** a valid teacher upload is committed but its passages have not yet been indexed
- **THEN** the upload keeps its existing success response and the search experience reports that indexing for the user's class is pending
- **AND** a subsequent successful indexing pass makes the new passage searchable

#### Scenario: Retry is idempotent

- **WHEN** indexing is retried after a worker restart or transient failure
- **THEN** each passage appears at most once for its current source version

### Requirement: Safe retrieval presentation

The web app MUST present search results and their source excerpts as non-executable content, including excerpts from Markdown materials, and MUST preserve the existing material-list search behavior.

#### Scenario: Untrusted passage is displayed

- **WHEN** a retrieved excerpt contains HTML or script-like text from an uploaded document
- **THEN** the search result and cited passage render without executing that content

#### Scenario: Material search remains available

- **WHEN** a user searches the existing material list by title or body
- **THEN** `/api/materials?q=...` retains its existing material-list behavior independently of knowledge search
