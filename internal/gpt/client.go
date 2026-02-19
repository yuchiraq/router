package gpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"router/internal/clog"
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
	clog.Infof("GPT: incoming request chat_id=%d enabled=%t", chatID, cfg.Enabled)
	if !cfg.Enabled {
		clog.Warnf("GPT: disabled in settings chat_id=%d", chatID)
		return "GPT выключен в настройках.", nil
	}
	if cfg.APIKey == "" {
		clog.Warnf("GPT: api key is empty chat_id=%d", chatID)
		return "Не задан OpenAI API key в настройках GPT.", nil
	}
	if !c.IsAllowedChat(chatID) {
		clog.Warnf("GPT: chat is not allowed chat_id=%d", chatID)
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
	clog.Debugf("GPT: sending request chat_id=%d model=%s user_text_len=%d", chatID, model, len(userText))

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
		clog.Errorf("GPT: request failed chat_id=%d err=%v", chatID, err)
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		clog.Errorf("GPT: openai non-2xx chat_id=%d status=%s body=%s", chatID, resp.Status, strings.TrimSpace(string(body)))
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
		clog.Errorf("GPT: failed to decode response chat_id=%d err=%v", chatID, err)
		return "", err
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		clog.Warnf("GPT: empty response chat_id=%d", chatID)
		return "Пустой ответ от модели.", nil
	}
	answer := strings.TrimSpace(out.Choices[0].Message.Content)
	clog.Infof("GPT: reply ready chat_id=%d answer_len=%d", chatID, len(answer))
	return answer, nil
}
