# Frontend Quality Guidelines

## Current Checks

The repository has no frontend package manager, bundler, linter, or browser test
suite. At minimum, syntax-check changed scripts with Node when available:

```bash
node --check web/login.js
node --check web/upload.js
```

For behavior changes, manually verify the matching page in a browser: empty and
invalid form input, request failure, successful action, keyboard focus, and
mobile-size layout. Upload changes must be checked against the server contract
for both an allowed image and an allowed document.

## Review Rules

- Keep user-visible messages in Chinese, matching the existing interface.
- Preserve the disabled/loading state around asynchronous actions.
- Reset stale previews with `URL.revokeObjectURL`.
- Use `textContent` for dynamic messages rather than inserting response data as
  HTML.
- Preserve authentication redirects on `401` and the stable `/api/images` form
  field name (`image`).
