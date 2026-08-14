# Backend Quality Guidelines

## Required Checks

Run `gofmt` on changed Go files, then run:

```bash
go test ./...
go vet ./...
```

For upload, authentication, or concurrency-sensitive changes, also run
`go test -race ./...`. The deployment contract additionally requires a static
Linux amd64 build before release.

## Test Pattern

Use `httptest.NewRequest` and `httptest.NewRecorder` against `a.Handler()`.
Create an isolated app with `newTestApp(t)`, which supplies `t.TempDir()` and a
fixed clock. Build multipart fixtures through `uploadRequest`; keep malformed
and spoofed payload cases beside successful uploads in `app_test.go`.

## Review Rules

- Maintain the public API, cookie, storage, and validation contract in
  `image-host-contract.md`.
- Treat filenames and MIME declarations as untrusted; derive the stored name
  from validated content and use exclusive creation.
- Bound body/file sizes before persistence and verify rejected uploads leave no
  files behind.
- Keep the standard-library-only dependency model unless a requirement demands
  an external library.
- Check shutdown behavior and error logging whenever server lifecycle code
  changes.
