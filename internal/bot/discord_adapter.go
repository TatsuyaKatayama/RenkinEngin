package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ToolCaller interface {
	CallTool(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error)
}

type DiscordAdapter struct {
	caller      ToolCaller
	channelID   string
	botUserID   string
	botUsername string
}

func NewDiscordAdapter(caller ToolCaller, channelID string, botUserID string) *DiscordAdapter {
	return &DiscordAdapter{
		caller:    caller,
		channelID: channelID,
		botUserID: botUserID,
	}
}

func (a *DiscordAdapter) SetBotUsername(username string) {
	a.botUsername = username
}

func (a *DiscordAdapter) PollSince(ctx context.Context, cp Checkpoint) ([]BoardItem, Checkpoint, error) {
	args := map[string]any{
		"channelId": a.channelID,
	}
	if cp.LastMessageID != "" {
		args["after"] = cp.LastMessageID
	}

	raw, err := a.caller.CallTool(ctx, "read_messages", args)
	if err != nil {
		return nil, cp, err
	}

	messages, err := decodeDiscordMessages(raw)
	if err != nil {
		return nil, cp, err
	}

	items := make([]BoardItem, 0, len(messages))
	next := cp
	for _, msg := range messages {
		if msg.ID == "" {
			continue
		}
		if compareSnowflake(msg.ID, next.LastMessageID) > 0 {
			next.LastMessageID = msg.ID
		}
		if a.isOwnBotMessage(msg) {
			continue
		}
		items = append(items, BoardItem{
			ID:        msg.ID,
			ChannelID: firstNonEmpty(msg.ChannelID, a.channelID),
			AuthorID:  msg.Author.ID,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		return compareSnowflake(items[i].ID, items[j].ID) < 0
	})
	return items, next, nil
}

func (a *DiscordAdapter) IsResolved(ctx context.Context, item BoardItem) (bool, error) {
	channelID := firstNonEmpty(item.ChannelID, a.channelID)
	args := map[string]any{
		"channelId": channelID,
		"count":     100,
	}

	raw, err := a.caller.CallTool(ctx, "read_messages", args)
	if err != nil {
		return false, err
	}

	messages, err := decodeDiscordMessages(raw)
	if err != nil {
		return false, err
	}
	for _, msg := range messages {
		if !a.isOwnBotMessage(msg) || msg.MessageReference == nil {
			continue
		}
		if msg.MessageReference.MessageID == item.ID {
			return true, nil
		}
	}
	return false, nil
}

func (a *DiscordAdapter) isOwnBotMessage(msg discordMessage) bool {
	if a.botUserID != "" && msg.Author.ID == a.botUserID {
		return true
	}
	if a.botUsername != "" && msg.Author.ID == a.botUsername {
		return true
	}
	return msg.Author.Bot
}

type discordMessage struct {
	ID        string    `json:"id"`
	ChannelID string    `json:"channel_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Author    struct {
		ID  string `json:"id"`
		Bot bool   `json:"bot"`
	} `json:"author"`
	MessageReference *struct {
		MessageID string `json:"message_id"`
	} `json:"message_reference"`
}

func decodeDiscordMessages(raw json.RawMessage) ([]discordMessage, error) {
	raw = unwrapMCPToolResult(raw)
	if text, ok := rawJSONString(raw); ok {
		if json.Valid([]byte(text)) {
			return decodeDiscordMessages(json.RawMessage(text))
		}
		return parseDiscordMessagesText(text), nil
	}

	var messages []discordMessage
	if err := json.Unmarshal(raw, &messages); err == nil {
		return messages, nil
	}

	var object struct {
		Messages []discordMessage `json:"messages"`
		Data     []discordMessage `json:"data"`
		Items    []discordMessage `json:"items"`
	}
	if err := json.Unmarshal(raw, &object); err != nil {
		return nil, fmt.Errorf("decode discord messages: %w", err)
	}
	switch {
	case object.Messages != nil:
		return object.Messages, nil
	case object.Data != nil:
		return object.Data, nil
	case object.Items != nil:
		return object.Items, nil
	default:
		return nil, fmt.Errorf("decode discord messages: response has no messages array")
	}
}

func rawJSONString(raw json.RawMessage) (string, bool) {
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return "", false
	}
	return text, true
}

var discordTextMessagePattern = regexp.MustCompile("(?s)- \\(ID: ([0-9]+)\\) \\*\\*\\[([^\\]]+)\\]\\*\\* `([^`]+)`: ```(.*?)```")

func parseDiscordMessagesText(text string) []discordMessage {
	matches := discordTextMessagePattern.FindAllStringSubmatch(text, -1)
	messages := make([]discordMessage, 0, len(matches))
	for _, match := range matches {
		var createdAt time.Time
		if parsed, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(match[3])); err == nil {
			createdAt = parsed
		}
		msg := discordMessage{
			ID:        match[1],
			Content:   strings.TrimSpace(match[4]),
			CreatedAt: createdAt,
		}
		msg.Author.ID = strings.TrimSpace(match[2])
		messages = append(messages, msg)
	}
	return messages
}

func unwrapMCPToolResult(raw json.RawMessage) json.RawMessage {
	var result struct {
		StructuredContent json.RawMessage `json:"structuredContent"`
		Content           []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return raw
	}
	if len(result.StructuredContent) > 0 {
		return result.StructuredContent
	}
	for _, content := range result.Content {
		if content.Type == "text" && json.Valid([]byte(content.Text)) {
			return json.RawMessage(content.Text)
		}
		if content.Type == "text" {
			if encoded, err := json.Marshal(content.Text); err == nil {
				return encoded
			}
		}
	}
	return raw
}

func compareSnowflake(a, b string) int {
	if a == b {
		return 0
	}
	ai, aErr := strconv.ParseUint(a, 10, 64)
	bi, bErr := strconv.ParseUint(b, 10, 64)
	if aErr == nil && bErr == nil {
		switch {
		case ai < bi:
			return -1
		case ai > bi:
			return 1
		default:
			return 0
		}
	}
	if a < b {
		return -1
	}
	return 1
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
