# Database Guidelines

## Current State

The application has no database, ORM, migrations, or query layer. Authentication
uses a password from environment configuration and an HMAC-signed cookie; uploads
are persisted as files under `UPLOAD_DIR`.

Do not introduce a database abstraction, migration tooling, or persistence model
for changes that fit the current local-file contract. Filename generation and
exclusive file creation in `internal/app/upload.go` are the persistence boundary.

If durable metadata or querying becomes a product requirement, design the storage
model and migration/backup story as a separate task before adding a dependency.
