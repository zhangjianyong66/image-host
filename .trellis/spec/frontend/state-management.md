# Frontend State Management

## Current Pattern

State is local to each page script. `web/upload.js` keeps the selected file,
its derived type, and an object URL in module variables, while visibility and
messages are rendered directly into the DOM. `web/login.js` reads the password
field on submit and disables the submit button while the request is pending.

Server interaction is request-driven: login uses `fetch`; upload uses
`XMLHttpRequest` to expose progress. No server cache, global store, URL state,
or persistence layer exists.

Keep only source state in variables and derive display details from it. Clean up
object URLs with `URL.revokeObjectURL` when replacing or clearing a preview.
Do not add global state for values used by one page.
