package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"go.uber.org/zap"

	"github.com/freQuensy23-coder/tgs/internal/config"
	"github.com/freQuensy23-coder/tgs/internal/sender"
)

const (
	defaultAppID   = 1308644
	defaultAppHash = "df0215899cd03b8c63cd70b7ed01b3ef"
)

func cmdLogin(ctx context.Context, mode string) error {
	switch mode {
	case "bot":
		return loginBot(ctx)
	case "user":
		return loginUser(ctx)
	default:
		return fmt.Errorf("unknown login mode %q, use: tgs login bot|user", mode)
	}
}

func loginBot(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("Bot token (from @BotFather): ")
	scanner.Scan()
	token := strings.TrimSpace(scanner.Text())
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	fmt.Print("Validating token... ")
	name, err := sender.ValidateToken(ctx, token)
	if err != nil {
		return err
	}
	fmt.Printf("OK, bot: %s\n", name)

	fmt.Print("Your numeric chat ID (send /start to @userinfobot to get it): ")
	scanner.Scan()
	chatIDStr := strings.TrimSpace(scanner.Text())
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid chat ID: %w", err)
	}

	cfg := &config.Config{
		Mode:        "bot",
		BotToken:    token,
		OwnerChatID: chatID,
	}
	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Println("Logged in as bot. Config saved to ~/.tgs/config.json")
	return nil
}

func loginUser(ctx context.Context) error {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Print("Phone number (with country code, e.g. +1234567890): ")
	scanner.Scan()
	phone := strings.TrimSpace(scanner.Text())
	if phone == "" {
		return fmt.Errorf("phone cannot be empty")
	}

	appID := defaultAppID
	appHash := defaultAppHash

	cfg := &config.Config{
		Mode:    "user",
		AppID:   appID,
		AppHash: appHash,
		Phone:   phone,
	}

	sessionPath := cfg.SessionPath()

	os.Remove(sessionPath)

	client := telegram.NewClient(appID, appHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: sessionPath},
		Logger:         zap.NewNop(),
	})

	flow := auth.NewFlow(
		terminalAuth{phone: phone, scanner: scanner},
		auth.SendCodeOptions{},
	)

	var displayName string
	err := client.Run(ctx, func(ctx context.Context) error {
		if err := client.Auth().IfNecessary(ctx, flow); err != nil {
			os.Remove(sessionPath)
			return fmt.Errorf("auth: %w", err)
		}

		self, err := client.Self(ctx)
		if err != nil {
			return fmt.Errorf("get self: %w", err)
		}

		displayName = strings.TrimSpace(self.FirstName + " " + self.LastName)
		return nil
	})
	if err != nil {
		return err
	}

	if err := cfg.Save(); err != nil {
		return err
	}

	fmt.Printf("Logged in as %s. Config saved to ~/.tgs/config.json\n", displayName)
	return nil
}

type terminalAuth struct {
	phone   string
	scanner *bufio.Scanner
}

func (a terminalAuth) Phone(_ context.Context) (string, error) {
	return a.phone, nil
}

func (a terminalAuth) Password(_ context.Context) (string, error) {
	fmt.Print("2FA password: ")
	a.scanner.Scan()
	return strings.TrimSpace(a.scanner.Text()), nil
}

func (a terminalAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	fmt.Print("Auth code: ")
	a.scanner.Scan()
	return strings.TrimSpace(a.scanner.Text()), nil
}

func (a terminalAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign up not supported, use an existing account")
}

func (a terminalAuth) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return nil
}
