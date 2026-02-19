package gpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"router/internal/storage"
	"strings"
	"time"
)

type Client struct {
	store      *storage.GPTStore
	httpClient *http.Client
}

func NewClient(store *storage.GPTStore) *Client {
	return &Client{
		store:      store,
		httpClient: &http.Client{Timeout: 40 * time.Second},
	}
}

func (c *Client) IsAllowedChat(chatID int64) bool {
	cfg := c.store.Get()
	if len(cfg.OnlyChatIDs) == 0 {
		return true
	}
	for _, id := range cfg.OnlyChatIDs {
		if id == chatID {
			return true
		}
	}
	return false
}

func (c *Client) Reply(chatID int64, userText string) (string, error) {
	cfg := c.store.Get()
	if !cfg.Enabled {
		return "GPT выключен в настройках.", nil
	}
	if cfg.APIKey == "" {
		return "Не задан OpenAI API key в настройках GPT.", nil
	}
	if !c.IsAllowedChat(chatID) {
		return "Этот чат не входит в список разрешённых для GPT.", nil
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = "gpt-4o-mini"
	}

	sys := strings.TrimSpace(cfg.SystemPrompt)
	if sys == "" {
		sys = "Ты помощник для администрирования reverse-proxy Router. Отвечай на русском языке коротко и по делу."
	}

	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": userText},
		},
	}
	b, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, "https://api.openai.com/v1/chat/completions", bytes.NewReader(b))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("openai error: %s %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "Пустой ответ от модели.", nil
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
