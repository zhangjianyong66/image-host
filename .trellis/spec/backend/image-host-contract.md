# Image Host Contract

## Scenario: Password-Protected Local Image Hosting

### 1. Scope / Trigger

Use this contract when changing authentication, image upload, public image URLs, local storage, or the `image.zhangjianyong.top` deployment. These behaviors cross browser, Go HTTP, filesystem, systemd, Nginx, and Certbot boundaries.

### 2. Signatures

- `POST /api/login` with JSON `{"password": string}`.
- `POST /api/logout` and `GET /api/session`.
- `POST /api/images` with one multipart file field named `image`.
- `GET /images/{32-lowercase-hex}.{jpg|png|gif|webp}` is public.
- `GET /healthz` returns `{"status":"ok"}`.

### 3. Contracts

- A successful upload returns HTTP `201` and `{"url":"<PUBLIC_BASE_URL>/images/<generated-name>"}`. The browser displays this exact URL and provides a copy action.
- Required environment keys: `UPLOAD_PASSWORD`, `SESSION_SECRET` (at least 32 characters). Production also sets `LISTEN_ADDR=127.0.0.1:18090`, `UPLOAD_DIR=/var/lib/image-host/uploads`, `PUBLIC_BASE_URL=https://image.zhangjianyong.top`, and `COOKIE_SECURE=true`.
- Session cookies are HMAC-signed, expire after 24 hours, and use `HttpOnly`, `Secure` in production, and `SameSite=Strict`.
- Production images live outside release directories. Nginx proxies to the Go service; it does not own upload persistence.

### 4. Validation & Error Matrix

| Condition | Status | Behavior |
| --- | --- | --- |
| Missing or invalid session on upload | `401` | JSON error; no body parsing or file write |
| Empty/malformed multipart request | `400` | JSON error; no file remains |
| More than one part or wrong field | `400` | Reject the request |
| Image content exceeds 10 MiB | `413` | Reject before persistence |
| MIME declaration unsupported | `415` | Allow only JPEG, PNG, GIF, WebP |
| File header and MIME disagree | `415` | Reject spoofed content |
| Invalid public stored name | `404` | Never resolve arbitrary filesystem paths |

### 5. Good/Base/Bad Cases

- Good: authenticated `image/png` with a PNG signature is saved under a generated name and returns an absolute HTTPS URL.
- Base: the same URL remains readable without a Cookie after service restart.
- Bad: `../../name.png`, SVG, an empty file, a fake MIME type, or an 11 MiB image never creates an upload file.

### 6. Tests Required

- Unit/integration: password success/failure, Cookie flags, tampered and expired tokens.
- Integration: upload all supported signatures, absolute response URL, anonymous public read, restart persistence, generated filename pattern, invalid type/size/field, and zero files after rejection.
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

- The ecs2 Nginx binary is `/usr/local/nginx/sbin/nginx`; do not use `systemctl reload nginx`.
- Back up `/usr/local/nginx/conf/conf.d/image.zhangjianyong.top.conf` before a switch and run `nginx -t` before reload.
- Never stop or replace `web-projects-hub-backend.service`; other domains share it.
- Keep `image.zhangjianyong.top:/usr/local/nginx/conf/conf.d/image.zhangjianyong.top.conf` in `/root/ssl_auto_renew/domains.conf` so the existing cron workflow includes this certificate.
