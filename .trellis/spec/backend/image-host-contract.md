# Image Host Contract

## Scenario: Password-Protected Local Image Hosting

### 1. Scope / Trigger

Use this contract when changing authentication, file upload, public file URLs, local storage, or the `image.zhangjianyong.top` deployment. These behaviors cross browser, Go HTTP, filesystem, systemd, Nginx, and Certbot boundaries.

### 2. Signatures

- `POST /api/login` with JSON `{"password": string}`.
- `POST /api/logout` and `GET /api/session`.
- `POST /api/images` with 1-9 repeated multipart file fields named `image`.
- `GET /images/{32-lowercase-hex}.{jpg|png|gif|webp|mp3|wav|ogg|m4a|mp4|webm|mov|pdf|epub|txt|md|docx|xlsx|pptx}` is public.
- `GET /healthz` returns `{"status":"ok"}`.

### 3. Contracts

- A processed batch returns HTTP `201` and `{"results":[{"filename":string,"status":"uploaded","url":string}|{"filename":string,"status":"failed","error":string}]}`. Results preserve request order; successful files remain when another item fails.
- Images are limited to 10 MiB, audio and documents to 50 MiB, and video to 200 MiB. Supported audio is MP3/WAV/OGG/M4A; supported video is MP4/WebM/MOV.
- Required environment keys: `UPLOAD_PASSWORD`, `SESSION_SECRET` (at least 32 characters). Production also sets `LISTEN_ADDR=127.0.0.1:18090`, `UPLOAD_DIR=/var/lib/image-host/uploads`, `PUBLIC_BASE_URL=https://image.zhangjianyong.top`, and `COOKIE_SECURE=true`.
- Session cookies are HMAC-signed, expire after 24 hours, and use `HttpOnly`, `Secure` in production, and `SameSite=Strict`.
- Production images live outside release directories. Nginx proxies to the Go service; it does not own upload persistence.

### 4. Validation & Error Matrix

| Condition | Status | Behavior |
| --- | --- | --- |
| Missing or invalid session on upload | `401` | JSON error; no body parsing or file write |
| Empty/malformed multipart request | `400` | JSON error; no file remains |
| More than 9 parts or wrong field | `400` | Reject the request |
| Item exceeds its media limit | `201` | Return an item-level `failed` result; do not persist it |
| MIME declaration unsupported | `201` | Return an item-level `failed` result |
| File signature, container structure, or UTF-8 text content disagrees with MIME | `201` | Return an item-level `failed` result |
| Invalid public stored name | `404` | Never resolve arbitrary filesystem paths |

### 5. Good/Base/Bad Cases

- Good: authenticated `image/png` with a PNG signature or a structurally valid declared document is saved under a generated name and returns an absolute HTTPS URL.
- Base: the same URL remains readable without a Cookie after service restart. PDF, TXT and MD open inline; EPUB and OpenXML documents download as attachments.
- Bad: `../../name.png`, SVG, an empty file, a fake MIME type, an 11 MiB image, a 51 MiB audio/document, or a 201 MiB video never creates an upload file.

### 6. Tests Required

- Unit/integration: password success/failure, Cookie flags, tampered and expired tokens.
- Integration: upload every supported image/document type, absolute response URL, anonymous public read, response MIME and disposition, restart persistence, generated filename pattern, invalid type/size/field, partial-success result ordering, nine-file limit, and zero files for failed items.
- Deployment: `go test -race ./...`, `go vet ./...`, static Linux amd64 build, systemd active/enabled, Nginx `-t`, HTTP-to-HTTPS redirect, domain upload/read smoke test, and public certificate expiry verification.

### 7. Wrong vs Correct

#### Wrong

```go
path := filepath.Join(uploadDir, multipartFile.Filename)
```

This trusts a client filename and allows collision or path traversal behavior.

#### Correct

```go
name := hex.EncodeToString(cryptoRandom16Bytes) + validatedExtension
file, err := os.OpenFile(filepath.Join(uploadDir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
```

The server owns the stored filename, validates the extension from file content, and uses exclusive creation.

## Infrastructure Boundary

- The ecs2 Nginx master runs `/usr/sbin/nginx -c /usr/local/nginx/conf/nginx.conf`. Pass this explicit config for `-t` and `-s reload`; do not use `systemctl reload nginx`, which targets the inactive default unit.
- Back up `/usr/local/nginx/conf/conf.d/image.zhangjianyong.top.conf` before a switch and run `nginx -t` before reload.
- Never stop or replace `web-projects-hub-backend.service`; other domains share it.
- Keep `image.zhangjianyong.top:/usr/local/nginx/conf/conf.d/image.zhangjianyong.top.conf` in `/root/ssl_auto_renew/domains.conf` so the existing cron workflow includes this certificate.


## Scenario: Token-Authenticated Media Uploads

### 1. Scope / Trigger

Programmatic uploads compatible with the NewAPI media client. Existing browser uploads keep their Cookie and partial-success contract.

### 2. Signatures

- POST /api/v1/media/images: exactly one multipart file field named `file`.
- POST /api/v1/media/images/batch: 1-9 repeated multipart file fields named `file`.
- Both require Authorization: Bearer <token>. Cookies and query parameters do not authenticate these routes.

### 3. Contracts

- Optional NEWAPI_MEDIA_UPLOAD_TOKENS is a comma-separated list. Trim surrounding whitespace; require at least 32 characters and valid Bearer token characters per item. Empty configuration disables Token upload. Empty entries or invalid tokens fail startup without exposing secrets.
- Production stores this key in /etc/image-host/image-host.env, loaded by systemd EnvironmentFile. Local .env must be explicitly exported by the shell; Go does not parse dotenv. Real .env files are Git-ignored.
- Single upload returns 201 with filename and url. Batch returns 201 with ordered items, each containing filename and url. All supported media/document types and per-file limits from the browser contract apply.
- Stage validated files privately inside UploadDir, then publish without overwriting filenames. Keep only one bounded file in memory. Failed requests clean staging files; publication failures roll back files published by this request. Existing public files remain untouched. This is request-level rollback, not crash-safe multi-file atomicity.
- Compare all configured tokens using the existing constant-time hash comparison. Never log tokens or put real tokens into tracked examples.

### 4. Validation & Error Matrix

| Condition | Status | Behavior |
| --- | --- | --- |
| Missing/invalid/duplicate Authorization or no configured tokens | 401 | JSON error and WWW-Authenticate: Bearer; no parsing or file write |
| Empty/malformed multipart, wrong field, empty file, too many files | 400 | JSON error; remove this request staging files |
| File or body exceeds limit | 413 | JSON error; no published files from request |
| Unsupported MIME or content mismatch | 415 | JSON error; no published files from request |
| Staging, persistence or publication failure | 500 | Generic JSON error; internal non-secret log and cleanup |

### 5. Good/Base/Bad Cases

- Good: any configured token uploads PNG via file; returned URL is anonymously readable. Nine valid files yield nine ordered URLs.
- Base: empty token configuration leaves password login and /api/images usable while new routes return 401.
- Bad: Cookie alone, query token, wrong token, an extra single-upload part, or an invalid batch item cannot publish files.

### 6. Tests Required

- Configuration: trimming, multiple tokens, disabled mode, empty entries, short/invalid tokens, no secret echo.
- Authentication: missing/invalid/duplicate headers, alternate configured token, Cookie rejection, legacy Cookie isolation.
- Integration: single/batch URLs, nine-file success, ten-file rejection, type/size/field errors, malformed trailing multipart, later-item cleanup, publication failure rollback, anonymous reads and legacy regressions.
- Checks: gofmt, go test, go vet, race tests, static Linux amd64 build, actual media-client smoke upload.

### 7. Wrong vs Correct

#### Wrong

Accepting UPLOAD_PASSWORD or image_host_session as a Bearer token, or returning legacy results to a client expecting items[].url.

#### Correct

Validate the Bearer credential against NEWAPI_MEDIA_UPLOAD_TOKENS and return the route-specific url or items[].url after validating all files.
