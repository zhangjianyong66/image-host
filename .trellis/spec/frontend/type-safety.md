# Frontend Type Safety

## Current State

Browser code is plain JavaScript; there is no TypeScript, generated API client,
or runtime schema library. Keep browser-side values defensive at the API
boundary. Both page scripts parse server responses cautiously and fall back to
an empty object when JSON parsing fails.

Validate user-facing file name, extension, and size early for feedback, but do
not treat that validation as a security boundary. The Go server owns MIME and
content validation. Keep frontend constants synchronized with the backend
contract: `web/upload.js` explicitly maps accepted extensions to request MIME
types and applies the image/document limits.

Avoid unchecked assumptions about response fields. Read `body.error` only as an
optional fallback message and use the server response URL only after a `201`.
