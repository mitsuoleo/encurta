package web

import "embed"

//go:embed index.html
var Index []byte

//go:embed openapi.yaml
var OpenAPI []byte

//go:embed app.css app.js logic.js qr.js
var Assets embed.FS
