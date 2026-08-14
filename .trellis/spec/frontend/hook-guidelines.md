# Frontend Hook Guidelines

## Current State

The frontend has no React, custom hooks, or client-side data-fetching library.
Stateful behavior is implemented with module-level variables and DOM event
listeners in the page script.

For a small behavior addition, follow `web/upload.js`: keep private state near
the DOM references, use named functions for reusable transitions such as
`resetPreviewURL` and `rejectFile`, and register listeners once during script
evaluation. Use `fetch` for simple JSON requests and `XMLHttpRequest` only where
upload progress is needed.

Do not add a hook abstraction or state library without an approved frontend
architecture change.
