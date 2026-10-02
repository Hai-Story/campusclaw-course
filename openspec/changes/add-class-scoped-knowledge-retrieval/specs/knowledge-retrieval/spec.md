## Purpose

让已登录的教师和学生通过关键字、向量及混合检索找到自己班级的知识切片，核对出处，并在确有依据时取得带编号引用的简短回答；所有路径遵守现有班级隔离边界。

## ADDED Requirements

### Requirement: Class-scoped knowledge search

The system MUST provide a separate knowledge-search API for authenticated teachers and students. It MUST derive the search class from the server-side session for every retrieval path and source lookup, return only content belonging to that class, and never accept a client-supplied class as authority.

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

### Requirement: Three retrieval modes

The system MUST support `keyword`, `vector`, and `hybrid` search modes, with `hybrid` as the default. Keyword search MUST use the class-filtered text index without calling the embedding or vector service. Vector search MUST reject candidates with cosine similarity below 0.35. Hybrid search MUST combine independently filtered keyword and vector rankings using reciprocal rank fusion with k=60, without adding their raw scores.

#### Scenario: Exact term and paraphrase

- **WHEN** a same-class user searches an exact term in `keyword` mode and a semantic paraphrase in `vector` mode
- **THEN** each mode returns matching ready passages through its own retrieval path
- **AND** keyword mode makes no embedding or vector-service request

#### Scenario: Hybrid ranks shared evidence

- **WHEN** a passage qualifies in both keyword and vector paths
- **THEN** hybrid mode ranks it using contributions from both paths, while a passage absent from one path receives no contribution from that path

#### Scenario: Vector service outage

- **WHEN** the vector or embedding dependency is unavailable
- **THEN** keyword mode can still return ready class-scoped results
- **AND** vector and hybrid modes return 503 rather than silently changing modes

### Requirement: Bounded search contract

The system MUST accept a non-empty UTF-8 search query, impose a documented length limit and bounded result count, and distinguish an empty match set from a failed search. A class-scoped query with no qualifying candidate MUST return the exact message `资料中未找到相关内容` and an empty hits list.

#### Scenario: Valid search has no matches

- **WHEN** a valid query has no matching indexed passage in the user's class and indexing is complete
- **THEN** the server returns 200 with empty `hits`, the fixed no-evidence message, and a ready index state

#### Scenario: Invalid query

- **WHEN** the query is blank after trimming or exceeds the documented length limit
- **THEN** the server returns 400 without executing a search

#### Scenario: Search dependency unavailable

- **WHEN** the retrieval dependency cannot serve a valid search
- **THEN** the server returns 503 with a generic retryable error and no passages from another class

### Requirement: Configurable source-preserving chunking

The system MUST retain the original uploaded body and support `auto`, `custom`, and `hierarchy` chunking for new uploads and teacher-triggered reindexing. The default and seed backfill MUST use `auto`: at most 800 Unicode characters with 80-character overlap, preferring paragraph, newline or sentence boundaries. `custom` MUST accept a 100–2000-character maximum, 0–50% overlap, newline/blank-line/period separators, and optional URL/email removal or whitespace collapse. `hierarchy` MUST split Markdown at `#`–`###` headings, retain headings in their sections, and apply the auto window to oversized sections.

#### Scenario: Reindex with another strategy

- **WHEN** a teacher reindexes a same-class material using a different valid strategy
- **THEN** the original file and knowledge body stay unchanged and the active chunks use the requested strategy
- **AND** obsolete chunk and vector IDs do not remain searchable

#### Scenario: Preprocessing changes offsets

- **WHEN** custom preprocessing changes the text before chunking
- **THEN** chunk offsets refer to the processed chunking text and the result identifies that offset basis
- **AND** the original material remains available for source inspection

### Requirement: Verifiable source references

Every returned passage MUST contain an excerpt taken from its MySQL chunk text, the material ID, material title, original filename, chunk index, and start/end character offsets with their basis (`original` or `processed`). The web app MUST let the user open the authorized material detail and inspect the cited excerpt and surrounding source when the offsets refer to the original body.

#### Scenario: Follow a result to its source

- **WHEN** a user selects a search result
- **THEN** the app opens the corresponding authenticated material detail and identifies the cited passage, or shows the processed excerpt beside the original when preprocessing changed offsets
- **AND** the user can access the existing authenticated download action

#### Scenario: Stale or cross-class source reference

- **WHEN** a user requests a citation whose source material is absent or belongs to another class
- **THEN** the source access returns the same 404 status and response body as an unknown material or citation
- **AND** the response reveals no source text or metadata

### Requirement: Evidence-gated short answer

The system MUST provide an authenticated non-streaming ask API. It MUST retrieve at most four ready same-class passages with hybrid search for the latest user question, and MUST call the dialogue gateway only when at least one passage qualifies. It MUST number citations in result order and allow the answer to cite only those numbers.

#### Scenario: Answer with citations

- **WHEN** a question has qualifying same-class passages
- **THEN** the dialogue gateway receives the question plus only their titles, indices and chunk text, and the response contains a short answer with `[1]`-style references aligned to its citations list

#### Scenario: Dialogue model omits inline references

- **WHEN** the configured `course-chat` model returns an answer without inline citation markers despite receiving numbered same-class evidence
- **THEN** the server appends clearly labelled retrieved source numbers to the answer and returns their authorized citation details
- **AND** it still rejects any model-supplied citation number outside this request's evidence list

#### Scenario: No evidence

- **WHEN** a question has no qualifying same-class passage
- **THEN** the API returns 200 with answer `资料中未找到相关内容` and empty citations
- **AND** the dialogue gateway is not called

#### Scenario: Client supplied instructions and history

- **WHEN** the caller sends bounded prior user/assistant turns and a forged system message or `class_id`
- **THEN** only the allowed history is appended after retrieval for context, while the forged system message and class declaration have no authority

### Requirement: Searchable ingestion and recovery

The system MUST index existing knowledge entries and new successful uploads, preserve their source mapping, and recover indexing after interruption without duplicating search results. MySQL MUST retain chunk text and Qdrant MUST hold only vectors and identifiers with point ID equal to chunk ID. A successfully stored upload MUST remain stored if indexing is delayed; users MUST be told when their class's index is still catching up. Only ready chunks are searchable.

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

#### Scenario: Material is deleted

- **WHEN** a teacher deletes a same-class material, including a seed material
- **THEN** its chunks and vector points are removed from the active index and cannot appear in search or ask citations
- **AND** later worker passes do not recreate the deleted entry

### Requirement: Safe retrieval presentation

The web app MUST present search hits, citations and their source excerpts as non-executable content, including excerpts from Markdown materials, offer separate knowledge-search and knowledge-answer navigation, three retrieval modes and an ask interaction, and preserve the existing material-list search behavior.

#### Scenario: Search and answer are separately discoverable

- **WHEN** an authenticated user opens the app on desktop or mobile
- **THEN** they can enter knowledge search and knowledge answering directly from navigation
- **AND** semantic search is identified as the embedding-backed retrieval mode

#### Scenario: Untrusted passage is displayed

- **WHEN** a retrieved excerpt contains HTML or script-like text from an uploaded document
- **THEN** the search result and cited passage render without executing that content

#### Scenario: Material search remains available

- **WHEN** a user searches the existing material list by title or body
- **THEN** `/api/materials?q=...` retains its existing material-list behavior independently of knowledge search

### Requirement: Knowledge-answer chatbot

The web app MUST list a “知识问答” entry in the desktop left navigation and offer an equivalent mobile entry. Its main area MUST present consecutive user and assistant messages, a multiline composer, and separately openable sources for each answer. It MUST use the existing authenticated `/api/ask` API and the server-side gateway and model configuration supplied through `.env`. Ask responses MUST expose the class-scoped `index_state` alongside the answer and citations. The browser MUST NOT receive gateway credentials.

#### Scenario: Consecutive questions

- **WHEN** a logged-in user opens knowledge answering and sends two valid questions
- **THEN** both user questions and their assistant answers remain visible with their respective citations
- **AND** the second request includes at most the latest six completed user/assistant messages, each bounded to 1000 Unicode codepoints
- **AND** each new question is limited to 200 Unicode codepoints and retrieval still uses only the latest question

#### Scenario: Keyboard and Unicode input

- **WHEN** the user enters a question in the multiline composer
- **THEN** Enter sends, Shift+Enter inserts a newline, and IME composition never triggers sending
- **AND** blank or overlong questions cannot be sent and the character count treats a supplementary Unicode character as one codepoint

#### Scenario: Pending, failed and stopped requests

- **WHEN** an answer is pending, fails, or is stopped by the user
- **THEN** the interface shows its state and prevents duplicate submission while pending
- **AND** failure or cancellation preserves the question and allows retrying the latest round without duplicating its user message or adding unfinished messages to history

#### Scenario: Conversation lifecycle

- **WHEN** the user switches away from knowledge answering, clears the conversation, logs out, or unloads the view
- **THEN** any pending ask request is cancelled and a late response cannot overwrite or restore cleared messages
- **AND** completed messages survive navigation within the current login, while clearing, logout and refresh discard the in-memory conversation

#### Scenario: No evidence and delayed indexing

- **WHEN** an ask response contains no evidence or reports a building or degraded class index
- **THEN** the conversation displays the fixed no-evidence answer without fabricated sources and displays any returned index warning with that round
