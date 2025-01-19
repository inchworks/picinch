// Embed templates

package web

import (
	"embed"
)

//go:embed template template-club template-solo static
var Files embed.FS
