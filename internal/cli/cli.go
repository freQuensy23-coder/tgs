package cli

import (
	"context"
	"fmt"
	"os"
)

func Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	if args[0] == "login" {
		if len(args) < 2 {
			return fmt.Errorf("usage: tgs login bot|user")
		}
		return cmdLogin(ctx, args[1])
	}

	switch len(args) {
	case 1:
		path := args[0]
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("%q: not found", path)
		}
		return cmdSend(ctx, "", path)

	case 2:
		target := args[0]
		path := args[1]
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("%q: not found", path)
		}
		return cmdSend(ctx, target, path)

	default:
		printUsage()
		return fmt.Errorf("too many arguments")
	}
}
