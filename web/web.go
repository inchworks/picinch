// Embed templates

package web

import (
	"embed"
)

//go:embed template static
var Files embed.FS
