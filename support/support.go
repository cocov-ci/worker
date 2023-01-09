package support

import (
	"embed"
)

//go:embed extractor.tar dummy.tar
var Scripts embed.FS
