# Frontend Directory Structure

## Current Layout

The frontend is a small, framework-free browser client embedded into the Go
binary:

```
web/login.html       Login page markup
web/login.js         Login-page behavior
web/upload.html      Upload page markup
web/upload.js        Upload-page behavior
web/styles.css       Shared page styling
web/embed.go         go:embed declaration
```

Page-specific markup and behavior stay paired by basename. Shared visual styles
belong in `web/styles.css`. `web/embed.go` must include every asset required by
the Go `embed.FS` handler.

Do not create a `src/`, component, hook, or build-tool structure for a focused
page change; no such framework exists in this project.
