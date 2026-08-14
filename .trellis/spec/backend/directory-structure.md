# Backend Directory Structure

## Current Layout

This is a small Go monolith using only the standard library. The module name is
`image-host` and packages are organized by ownership, not by HTTP endpoint.

```
cmd/image-host/main.go       Process entry point and graceful shutdown
internal/app/                HTTP application, configuration, sessions, uploads, tests
web/                         Embedded browser assets and embed declaration
```

`cmd/image-host/main.go` assembles configuration, a text `slog` logger, and the
`app.App` HTTP handler. It must stay free of request handling and upload logic.
`internal/app` is intentionally private: add application behavior there unless
it must be imported by another Go module.

## Module Organization

- Keep HTTP route registration and page/API handlers in `internal/app/app.go`.
- Keep configuration parsing and validation in `internal/app/config.go`.
- Keep session token creation and verification in `internal/app/session.go`.
- Keep upload type validation, persistence, and public file serving in
  `internal/app/upload.go`.
- Put package integration tests beside the implementation in
  `internal/app/app_test.go`; tests use `httptest` and a temporary upload
  directory.
- Add browser assets under `web/` and expose them through `web/embed.go` using
  `go:embed`; do not serve files from the source checkout at runtime.

## Naming

Use lowercase Go filenames grouped by concern. Export only the process-facing
types and constructors that need to cross package boundaries, as `Config`,
`App`, `New`, and `Handler` do. Keep handler and helper functions unexported,
such as `handleLogin` and `persistImage`.

## Example

`internal/app/app.go` registers all routes in `(*App).Handler`, while
`internal/app/upload.go` owns the file-specific helpers called by its upload
and public-read handlers. This keeps the route surface and persistence rules
cohesive without introducing a service/repository layer that the project does
not use.
