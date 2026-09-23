# Auth, authorization and material ingestion delta

## ADDED Requirements

### Requirement: R1 Server-side authentication

The system MUST authenticate username/password credentials using bcrypt, issue a fresh opaque server-side session on success, and expose the current identity through `/api/me`.

#### Scenario: Successful login rotates the session

- **WHEN** a valid user logs in while presenting any prior session cookie
- **THEN** the server invalidates the prior session, creates a new unpredictable session, and sets an `HttpOnly; SameSite=Lax` cookie
- **AND** `/api/me` returns the current user, role and class from server-side data

#### Scenario: Credential failures are indistinguishable

- **WHEN** the username is unknown, the password is wrong, or the username+IP key is locked
- **THEN** the server returns the same status and message and performs an equivalent password-hash comparison

#### Scenario: Unauthenticated API access is denied

- **WHEN** a caller without a valid session requests a protected API
- **THEN** the server returns 401 and the response contains no material title, body or storage path

### Requirement: R2 Role-based upload authorization

The system MUST permit teachers to upload and MUST keep students read-only.

#### Scenario: Student upload is rejected

- **WHEN** a student submits a valid material upload
- **THEN** the server returns 403
- **AND** no file, material row or knowledge row is created

#### Scenario: Client role declarations are ignored

- **WHEN** a student declares a teacher role in a query, header or form field
- **THEN** the upload remains forbidden because the role comes from the server session

### Requirement: R3 Class isolation

The system MUST derive the tenant only from the authenticated session and enforce it independently on collection and object paths.

#### Scenario: Cross-class object access is hidden

- **WHEN** a class A user requests a class B material detail or file by ID
- **THEN** the server returns the same 404 status and body used for an unknown ID
- **AND** the response reveals no title, body or storage path

#### Scenario: Client class declarations are ignored

- **WHEN** a request supplies a different `class_id` in query, header or form data
- **THEN** list results and newly uploaded records still use the session class

### Requirement: R4 Validated atomic ingestion

The system MUST accept only non-empty UTF-8 `.txt` and `.md` files within the configured limit, generate storage names server-side, and ingest material metadata plus knowledge content atomically.

#### Scenario: Valid teacher upload is ingested

- **WHEN** a teacher uploads a valid allowed file
- **THEN** the server returns 201, stores the file, and commits one `materials` row and one `knowledge_entries` row for the session class

#### Scenario: Invalid or failed upload leaves no residue

- **WHEN** the extension is disallowed, the file is oversized, the body is empty/invalid UTF-8, or database ingestion fails
- **THEN** the server returns the corresponding 400, 413, or 5xx response
- **AND** no orphan database row or file remains

### Requirement: R5 Authenticated material access

The system MUST provide database-backed class-scoped list/search, detail and authenticated file download APIs without exposing the upload directory as static content.

#### Scenario: Same-class users can read and download

- **WHEN** an authenticated teacher or student requests a material in their class
- **THEN** the list/detail data comes from the database and the file is served only after authorization

### Requirement: R6 Idempotent seed data

The system MUST seed distinguishable A/B class data and the prescribed teacher/student accounts without duplicating or overwriting uploaded content on restart.

#### Scenario: Initialization runs twice

- **WHEN** database initialization runs more than once
- **THEN** user and seed-material counts remain stable and user-uploaded data remains unchanged

### Requirement: R7 Externalized secrets and password storage

The system MUST read secrets and seed passwords only from environment variables, MUST fail startup when required values are missing, and MUST store passwords only as bcrypt hashes.

#### Scenario: Required secret is missing

- **WHEN** `SESSION_SECRET` or a required database credential is absent
- **THEN** startup fails without using an embedded fallback

### Requirement: R8 Reproducible single-instance deployment

The system MUST run as web, api and db Compose services, expose only web, persist database/uploads, wait for database readiness, and provide a public liveness-only `/health` endpoint.

#### Scenario: Compose is recreated without deleting volumes

- **WHEN** operators run `docker compose down` and then start again without `-v`
- **THEN** database rows and uploaded files remain available

### Requirement: R9 Web behavior and safety

The web app MUST contain login and material experiences, restore identity with `/api/me`, handle 401 by returning to login, gate upload presentation by server-provided role, and use authenticated detail/download APIs.

#### Scenario: Browser session is restored safely

- **WHEN** the page is refreshed with a valid or expired session
- **THEN** the app decides the route from `/api/me`, never from a locally stored role

#### Scenario: Markdown HTML is not executed

- **WHEN** a Markdown material contains raw script/HTML
- **THEN** the detail view renders it as non-executable content

### Requirement: R10 Required material-page experience

The web app MUST use a PKU-red visual system and provide light/dark themes, responsive list/grid views, class-scoped search, a Ctrl/Cmd+K command palette, upload progress, toast feedback, and GFM Markdown rendering.

#### Scenario: Required controls remain usable

- **WHEN** a user switches theme/view, searches, opens the command palette, uploads, or reads Markdown on desktop or mobile
- **THEN** each interaction provides visible feedback and remains keyboard accessible

