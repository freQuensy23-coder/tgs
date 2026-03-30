package sender

import (
	"context"
	"fmt"

	"github.com/freQuensy23-coder/tgs/internal/config"
)

// Target represents the destination chat. Empty Name means Saved Messages / self.
type Target struct {
	Name string
}

// Sender sends files to Telegram chats.
type Sender interface {
	SendFile(ctx context.Context, target Target, filePath string) error
	Close() error
}

// New creates a Sender based on the config mode.
func New(ctx context.Context, cfg *config.Config) (Sender, error) {
	switch cfg.Mode {
	case "bot":
		return newBotSender(cfg)
	case "user":
		return newUserSender(ctx, cfg)
	default:
		return nil, fmt.Errorf("unknown mode %q, run: tgs login bot|user", cfg.Mode)
	}
}
