package storage

import (
	"fmt"
	"github.com/urfave/cli/v2"
)

func Initialize(mode string, ctx *cli.Context) (Base, error) {
	switch mode {
	case "local":
		cmdLineArg := "gs-local-storage-path"
		if !ctx.IsSet(cmdLineArg) {
			return nil, fmt.Errorf("%s: Must be set for storage mode 'local'", cmdLineArg)
		}
		return NewLocal(ctx.String(cmdLineArg))
	case "s3":
		cmdLineArg := "gs-s3-bucket-name"
		if !ctx.IsSet(cmdLineArg) {
			return nil, fmt.Errorf("%s: Must be set for storage mode 's3'", cmdLineArg)
		}
		return NewS3(ctx.String(cmdLineArg))
	default:
		return nil, fmt.Errorf("unknown storage mode %s", mode)
	}
}
