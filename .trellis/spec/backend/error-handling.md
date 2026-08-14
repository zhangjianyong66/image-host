# Backend Error Handling

## Error Boundaries

Handlers return client errors through the shared JSON response helpers in
`internal/app/app.go` and log unexpected server-side failures with `a.logger`.
Configuration and startup failures are returned as `error` from constructors or
validation and terminate from `main` before the server starts.

Use `errors.New` or `fmt.Errorf` with useful context when returning an error.
Do not expose filesystem paths, session secrets, or internal implementation
details in JSON responses.

## API Responses

Successful JSON responses are written with `writeJSON`; failures use
`writeError`, which returns the stable shape:

```json
{"error":"human-readable Chinese message"}
```

Preserve the status behavior in `image-host-contract.md`: malformed user input
is `400`, unauthenticated requests are `401`, unsupported/spoofed file content
is `415`, and oversized input is `413`. Validate before persisting so rejected
uploads leave no file behind.

## Patterns to Keep

- Bound request bodies before decoding, as login does with `http.MaxBytesReader`.
- Check and return failures at the operation that can handle them; do not ignore
  I/O, JSON encoding, or file-close errors unless cleanup is explicitly best
  effort.
- Log server failures with contextual fields, then return a generic `500` error.
- Do not use `panic` for request errors.
