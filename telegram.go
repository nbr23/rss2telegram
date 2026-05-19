package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TelegramClient struct {
	Token  string
	ChatID string
	HTTP   *http.Client
}

func NewTelegramClient(token, chatID string) *TelegramClient {
	return &TelegramClient{
		Token:  token,
		ChatID: chatID,
		HTTP:   &http.Client{Timeout: 30 * time.Second},
	}
}

func FormatMessage(feedTitle, articleTitle, articleURL string, published time.Time) string {
	var datePart string
	if !published.IsZero() {
		datePart = fmt.Sprintf(" <i>%s</i>", html.EscapeString(published.UTC().Format("2006-01-02 15:04 MST")))
	}
	return fmt.Sprintf(
		"<b>%s</b>%s\n<a href=\"%s\">%s</a>",
		html.EscapeString(feedTitle),
		datePart,
		html.EscapeString(articleURL),
		html.EscapeString(articleTitle),
	)
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func (t *TelegramClient) SendMessage(ctx context.Context, text string) error {
	return t.send(ctx, text, true)
}

func (t *TelegramClient) send(ctx context.Context, text string, allowRetry bool) error {
	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.Token)
	form := url.Values{}
	form.Set("chat_id", t.ChatID)
	form.Set("text", text)
	form.Set("parse_mode", "HTML")
	form.Set("disable_web_page_preview", "false")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := t.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("telegram request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusTooManyRequests && allowRetry {
		var tr telegramResponse
		_ = json.Unmarshal(body, &tr)
		wait := time.Duration(tr.Parameters.RetryAfter) * time.Second
		if wait <= 0 {
			wait = 5 * time.Second
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
		return t.send(ctx, text, false)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var tr telegramResponse
		_ = json.Unmarshal(body, &tr)
		desc := tr.Description
		if desc == "" {
			desc = string(body)
		}
		return fmt.Errorf("telegram %d: %s", resp.StatusCode, desc)
	}

	return nil
}
