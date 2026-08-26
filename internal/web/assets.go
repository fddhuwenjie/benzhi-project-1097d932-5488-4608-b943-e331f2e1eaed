package web

import "embed"

//go:embed assets/index.html assets/app.css assets/app.js
var assets embed.FS
