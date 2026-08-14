# Frontend Component Guidelines

## Current Pattern

The project does not use a component framework. HTML documents are the UI
structure and their paired JavaScript files attach behavior by querying stable
element IDs. For example, `web/upload.js` collects its page elements at module
scope with `document.querySelector('#file-input')` and updates `hidden`, text,
and form control properties in response to events.

When extending a page:

- Add semantic HTML with a unique, descriptive ID for elements JavaScript owns.
- Keep page behavior in the matching `.js` file and shared presentation in
  `styles.css`.
- Use native controls (`button`, `input`, `form`, labels) instead of clickable
  generic elements.
- Preserve existing `hidden`-based state transitions rather than duplicating
  page markup.
- Include accessible labels and keep focus behavior deliberate after validation,
  copy, and navigation actions.

There are no props, component composition, or CSS modules in the current code.
