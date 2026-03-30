package sender

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"go.uber.org/zap"

	"github.com/freQuensy23-coder/tgs/internal/config"
)

type UserSender struct {
	client *telegram.Client
	api    *tg.Client
	cancel context.CancelFunc
	done   chan error
}

func newUserSender(ctx context.Context, cfg *config.Config) (*UserSender, error) {
	client := telegram.NewClient(cfg.AppID, cfg.AppHash, telegram.Options{
		SessionStorage: &session.FileStorage{Path: cfg.SessionPath()},
		Logger:         zap.NewNop(),
	})

	us := &UserSender{client: client}
	runCtx, cancel := context.WithCancel(ctx)
	us.cancel = cancel
	us.done = make(chan error, 1)
	ready := make(chan struct{})

	go func() {
		us.done <- client.Run(runCtx, func(ctx context.Context) error {
			us.api = client.API()

			status, err := client.Auth().Status(ctx)
			if err != nil {
				return fmt.Errorf("auth status: %w", err)
			}
			if !status.Authorized {
				return fmt.Errorf("not authorized, run: tgs login user")
			}

			close(ready)
			<-ctx.Done()
			return ctx.Err()
		})
	}()

	select {
	case <-ready:
		return us, nil
	case err := <-us.done:
		cancel()
		return nil, fmt.Errorf("client error: %w", err)
	case <-ctx.Done():
		cancel()
		return nil, ctx.Err()
	}
}

func (u *UserSender) SendFile(ctx context.Context, target Target, filePath string) error {
	up := uploader.NewUploader(u.api)

	fmt.Fprintf(os.Stderr, "Uploading %s...\n", filePath)
	upload, err := up.FromPath(ctx, filePath)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	doc := message.UploadedDocument(upload).Filename(filepath.Base(filePath))

	sender := message.NewSender(u.api)

	var req *message.RequestBuilder
	if target.Name == "" {
		req = sender.Self()
	} else {
		req, err = u.resolveTarget(ctx, sender, target)
		if err != nil {
			return fmt.Errorf("resolve target: %w", err)
		}
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err = req.Media(ctx, doc)
		if err == nil {
			return nil
		}

		if wait, ok := tgerr.AsFloodWait(err); ok {
			fmt.Fprintf(os.Stderr, "Rate limited, waiting %v...\n", wait)
			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return fmt.Errorf("send: %w", err)
	}

	return fmt.Errorf("max retries exceeded")
}

func (u *UserSender) Close() error {
	u.cancel()
	err := <-u.done
	if err != nil && err.Error() == "context canceled" {
		return nil
	}
	return err
}

func (u *UserSender) resolveTarget(ctx context.Context, sender *message.Sender, target Target) (*message.RequestBuilder, error) {
	if req, found, err := u.findInDialogs(ctx, sender, target.Name); err != nil {
		return nil, err
	} else if found {
		return req, nil
	}

	name := target.Name
	if !strings.HasPrefix(name, "@") {
		name = "@" + name
	}
	return sender.Resolve(name), nil
}

func (u *UserSender) findInDialogs(ctx context.Context, sender *message.Sender, name string) (*message.RequestBuilder, bool, error) {
	nameLower := strings.ToLower(name)
	nameNoAt := strings.TrimPrefix(nameLower, "@")

	dialogs, err := u.api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      50,
	})
	if err != nil {
		return nil, false, fmt.Errorf("get dialogs: %w", err)
	}

	var (
		users    []tg.UserClass
		chats    []tg.ChatClass
		dialogList []tg.DialogClass
	)

	switch d := dialogs.(type) {
	case *tg.MessagesDialogs:
		users = d.Users
		chats = d.Chats
		dialogList = d.Dialogs
	case *tg.MessagesDialogsSlice:
		users = d.Users
		chats = d.Chats
		dialogList = d.Dialogs
	default:
		return nil, false, nil
	}

	userMap := make(map[int64]*tg.User)
	for _, u := range users {
		if user, ok := u.(*tg.User); ok {
			userMap[user.ID] = user
		}
	}

	chatMap := make(map[int64]tg.ChatClass)
	for _, c := range chats {
		switch ch := c.(type) {
		case *tg.Chat:
			chatMap[ch.ID] = ch
		case *tg.Channel:
			chatMap[ch.ID] = ch
		}
	}

	for _, dlg := range dialogList {
		d, ok := dlg.(*tg.Dialog)
		if !ok {
			continue
		}

		var title, username string
		var inputPeer tg.InputPeerClass

		switch p := d.Peer.(type) {
		case *tg.PeerUser:
			user, ok := userMap[p.UserID]
			if !ok {
				continue
			}
			title = strings.TrimSpace(user.FirstName + " " + user.LastName)
			username = user.Username
			inputPeer = &tg.InputPeerUser{UserID: user.ID, AccessHash: user.AccessHash}

		case *tg.PeerChat:
			ch, ok := chatMap[p.ChatID]
			if !ok {
				continue
			}
			if chat, ok := ch.(*tg.Chat); ok {
				title = chat.Title
				inputPeer = &tg.InputPeerChat{ChatID: chat.ID}
			}

		case *tg.PeerChannel:
			ch, ok := chatMap[p.ChannelID]
			if !ok {
				continue
			}
			if channel, ok := ch.(*tg.Channel); ok {
				title = channel.Title
				username = channel.Username
				inputPeer = &tg.InputPeerChannel{ChannelID: channel.ID, AccessHash: channel.AccessHash}
			}
		}

		if inputPeer == nil {
			continue
		}

		titleLower := strings.ToLower(title)
		usernameLower := strings.ToLower(username)

		if titleLower == nameLower ||
			usernameLower == nameNoAt ||
			(username != "" && strings.Contains(titleLower, nameLower)) {
			return sender.To(inputPeer), true, nil
		}
	}

	return nil, false, nil
}
