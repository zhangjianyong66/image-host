package webassets

import "embed"

// Files contains the complete browser UI and is linked into the executable.
//
//go:embed *.html *.css *.js
var Files embed.FS
