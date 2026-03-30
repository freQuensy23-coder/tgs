package sender

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/freQuensy23-coder/tgs/internal/config"
)

const (
	botAPIBase     = "https://api.telegram.org/bot"
	botMaxFileSize = 50 * 1024 * 1024 // 50 MB
	maxRetries     = 3
)

type apiResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
	ErrorCode   int             `json:"error_code"`
	Parameters  *apiParameters  `json:"parameters,omitempty"`
}

type apiParameters struct {
	RetryAfter int `json:"retry_after"`
}

type BotSender struct {
	token       string
	ownerChatID int64
	client      *http.Client
}

func newBotSender(cfg *config.Config) (*BotSender, error) {
	return &BotSender{
		token:       cfg.BotToken,
		ownerChatID: cfg.OwnerChatID,
		client:      &http.Client{Timeout: 5 * time.Minute},
	}, nil
}

func (b *BotSender) SendFile(ctx context.Context, target Target, filePath string) error {
	fi, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}
	if fi.Size() > botMaxFileSize {
		return fmt.Errorf("file is %d MB, bot API limit is 50 MB; use 'tgs login user' for files up to 2 GB",
			fi.Size()/(1024*1024))
	}

	chatID, err := b.resolveTarget(ctx, target)
	if err != nil {
		return fmt.Errorf("resolve target: %w", err)
	}

	return b.sendDocument(ctx, chatID, filePath)
}

func (b *BotSender) Close() error { return nil }

func (b *BotSender) resolveTarget(ctx context.Context, target Target) (int64, error) {
	if target.Name == "" {
		return b.ownerChatID, nil
	}

	username := target.Name
	if username[0] != '@' {
		username = "@" + username
	}

	body := fmt.Sprintf(`{"chat_id":"%s"}`, username)
	resp, err := b.apiCall(ctx, "getChat", "application/json", []byte(body))
	if err != nil {
		return 0, fmt.Errorf("getChat: %w", err)
	}

	var chat struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(resp, &chat); err != nil {
		return 0, fmt.Errorf("parse chat: %w", err)
	}
	return chat.ID, nil
}

func (b *BotSender) sendDocument(ctx context.Context, chatID int64, filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)

	go func() {
		var writeErr error
		defer func() {
			pw.CloseWithError(writeErr)
		}()

		if writeErr = writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); writeErr != nil {
			return
		}

		part, err := writer.CreateFormFile("document", filepath.Base(filePath))
		if err != nil {
			writeErr = err
			return
		}

		if _, err := io.Copy(part, file); err != nil {
			writeErr = err
			return
		}

		writeErr = writer.Close()
	}()

	url := botAPIBase + b.token + "/sendDocument"

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			file.Close()
			file, err = os.Open(filePath)
			if err != nil {
				pr.Close()
				return fmt.Errorf("reopen file: %w", err)
			}

			pr, pw = io.Pipe()
			retryWriter := multipart.NewWriter(pw)
			writer = retryWriter
			go func() {
				var writeErr error
				defer func() {
					pw.CloseWithError(writeErr)
				}()
				if writeErr = retryWriter.WriteField("chat_id", strconv.FormatInt(chatID, 10)); writeErr != nil {
					return
				}
				part, err := retryWriter.CreateFormFile("document", filepath.Base(filePath))
				if err != nil {
					writeErr = err
					return
				}
				if _, err := io.Copy(part, file); err != nil {
					writeErr = err
					return
				}
				writeErr = retryWriter.Close()
			}()
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, pr)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := b.client.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var apiResp apiResponse
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		if apiResp.OK {
			return nil
		}

		if apiResp.ErrorCode == 429 && apiResp.Parameters != nil && apiResp.Parameters.RetryAfter > 0 {
			wait := time.Duration(apiResp.Parameters.RetryAfter) * time.Second
			fmt.Fprintf(os.Stderr, "Rate limited, waiting %v...\n", wait)
			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		return fmt.Errorf("telegram API error %d: %s", apiResp.ErrorCode, apiResp.Description)
	}

	return fmt.Errorf("max retries exceeded")
}

func (b *BotSender) apiCall(ctx context.Context, method, contentType string, body []byte) (json.RawMessage, error) {
	url := botAPIBase + b.token + "/" + method

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "POST", url, io.NopCloser(
			io.NewSectionReader(readerAt(body), 0, int64(len(body))),
		))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", contentType)

		resp, err := b.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("send request: %w", err)
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var apiResp apiResponse
		if err := json.Unmarshal(respBody, &apiResp); err != nil {
			return nil, fmt.Errorf("parse response: %w", err)
		}

		if apiResp.OK {
			return apiResp.Result, nil
		}

		if apiResp.ErrorCode == 429 && apiResp.Parameters != nil && apiResp.Parameters.RetryAfter > 0 {
			wait := time.Duration(apiResp.Parameters.RetryAfter) * time.Second
			fmt.Fprintf(os.Stderr, "Rate limited, waiting %v...\n", wait)
			select {
			case <-time.After(wait):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		return nil, fmt.Errorf("API error %d: %s", apiResp.ErrorCode, apiResp.Description)
	}

	return nil, fmt.Errorf("max retries exceeded")
}

func ValidateToken(ctx context.Context, token string) (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	url := botAPIBase + token + "/getMe"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if !apiResp.OK {
		return "", fmt.Errorf("invalid token: %s", apiResp.Description)
	}

	var bot struct {
		FirstName string `json:"first_name"`
		Username  string `json:"username"`
	}
	if err := json.Unmarshal(apiResp.Result, &bot); err != nil {
		return "", fmt.Errorf("parse bot info: %w", err)
	}

	return fmt.Sprintf("%s (@%s)", bot.FirstName, bot.Username), nil
}

type readerAt []byte

func (r readerAt) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(r)) {
		return 0, io.EOF
	}
	n := copy(p, r[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
