# Backend Logging Guidelines

## Current Pattern

The service uses the standard-library `log/slog` package. `cmd/image-host/main.go`
creates a text handler writing to stderr and passes the logger to `app.New`.
Tests may pass `nil`; `New` then installs a discard logger.

Use structured key/value fields rather than string formatting for operational
data:

```go
logger.Info("image host started", "address", cfg.ListenAddr, "upload_dir", cfg.UploadDir)
logger.Error("graceful shutdown failed", "error", err)
```

Use `Info` for lifecycle events, `Error` for failed startup, shutdown, and
unexpected server failures. The current codebase does not use custom levels or
a request-logging middleware; do not add either without a concrete operational
need.

Never log upload passwords, session secrets, raw cookie values, or full uploaded
content. Log errors and non-sensitive operational identifiers only.
