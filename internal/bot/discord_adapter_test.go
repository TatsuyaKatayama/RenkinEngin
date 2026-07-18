package bot

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeToolCaller struct {
	name      string
	arguments map[string]any
	response  json.RawMessage
}

func (f *fakeToolCaller) CallTool(_ context.Context, name string, arguments map[string]any) (json.RawMessage, error) {
	f.name = name
	f.arguments = arguments
	return f.response, nil
}

func TestDiscordAdapterPollSinceUpdatesCheckpoint(t *testing.T) {
	caller := &fakeToolCaller{
		response: json.RawMessage(`{
			"content": [{
				"type": "text",
				"text": "{\"messages\":[{\"id\":\"100\",\"channel_id\":\"c1\",\"content\":\"older\",\"author\":{\"id\":\"u1\",\"bot\":false}},{\"id\":\"105\",\"channel_id\":\"c1\",\"content\":\"newer\",\"author\":{\"id\":\"u2\",\"bot\":false}}]}"
			}]
		}`),
	}
	adapter := NewDiscordAdapter(caller, "c1", "bot1")

	items, next, err := adapter.PollSince(context.Background(), Checkpoint{})

	require.NoError(t, err)
	assert.Equal(t, "read_messages", caller.name)
	assert.Equal(t, "c1", caller.arguments["channelId"])
	assert.NotContains(t, caller.arguments, "after")
	assert.Len(t, items, 2)
	assert.Equal(t, "100", items[0].ID)
	assert.Equal(t, "105", items[1].ID)
	assert.Equal(t, "105", next.LastMessageID)
}

func TestDiscordAdapterPollSincePassesCheckpointAndKeepsItWhenNoItems(t *testing.T) {
	caller := &fakeToolCaller{
		response: json.RawMessage(`{"messages":[]}`),
	}
	adapter := NewDiscordAdapter(caller, "c1", "bot1")

	items, next, err := adapter.PollSince(context.Background(), Checkpoint{LastMessageID: "105"})

	require.NoError(t, err)
	assert.Empty(t, items)
	assert.Equal(t, "105", caller.arguments["after"])
	assert.Equal(t, "105", next.LastMessageID)
}

func TestDiscordAdapterPollSinceExcludesBotMessages(t *testing.T) {
	caller := &fakeToolCaller{
		response: json.RawMessage(`{"messages":[
			{"id":"101","content":"human","author":{"id":"u1","bot":false}},
			{"id":"102","content":"bot flag","author":{"id":"other-bot","bot":true}},
			{"id":"103","content":"own id","author":{"id":"bot1","bot":false}}
		]}`),
	}
	adapter := NewDiscordAdapter(caller, "c1", "bot1")

	items, next, err := adapter.PollSince(context.Background(), Checkpoint{LastMessageID: "100"})

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "101", items[0].ID)
	assert.Equal(t, "103", next.LastMessageID)
}

func TestDiscordAdapterPollSinceDecodesStructuredContent(t *testing.T) {
	caller := &fakeToolCaller{
		response: json.RawMessage(`{
			"structuredContent": {
				"data": [
					{"id":"201","content":"from structured content","author":{"id":"u1","bot":false}}
				]
			}
		}`),
	}
	adapter := NewDiscordAdapter(caller, "c1", "")

	items, next, err := adapter.PollSince(context.Background(), Checkpoint{})

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "201", items[0].ID)
	assert.Equal(t, "201", next.LastMessageID)
}

func TestDiscordAdapterPollSinceDecodesJSONEncodedText(t *testing.T) {
	caller := &fakeToolCaller{
		response: json.RawMessage(`{
			"content": [{
				"type": "text",
				"text": "\"{\\\"messages\\\":[{\\\"id\\\":\\\"301\\\",\\\"content\\\":\\\"double encoded\\\",\\\"author\\\":{\\\"id\\\":\\\"u1\\\",\\\"bot\\\":false}}]}\""
			}]
		}`),
	}
	adapter := NewDiscordAdapter(caller, "c1", "")

	items, next, err := adapter.PollSince(context.Background(), Checkpoint{})

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "301", items[0].ID)
	assert.Equal(t, "double encoded", items[0].Content)
	assert.Equal(t, "301", next.LastMessageID)
}

func TestDiscordAdapterPollSinceDecodesFormattedText(t *testing.T) {
	text, err := json.Marshal("**Retrieved 2 messages:** \n- (ID: 401) **[alice]** `2026-07-18T05:03:04.167Z`: ```hello```\n- (ID: 405) **[bob]** `2026-07-18T05:04:04.167Z`: ```multi\nline```")
	require.NoError(t, err)
	caller := &fakeToolCaller{
		response: json.RawMessage(`{"content":[{"type":"text","text":` + string(text) + `}]}`),
	}
	adapter := NewDiscordAdapter(caller, "c1", "")

	items, next, err := adapter.PollSince(context.Background(), Checkpoint{})

	require.NoError(t, err)
	require.Len(t, items, 2)
	assert.Equal(t, "401", items[0].ID)
	assert.Equal(t, "alice", items[0].AuthorID)
	assert.Equal(t, "hello", items[0].Content)
	assert.Equal(t, "405", items[1].ID)
	assert.Equal(t, "multi\nline", items[1].Content)
	assert.Equal(t, "405", next.LastMessageID)
}

func TestDiscordAdapterPollSinceExcludesFormattedBotUsername(t *testing.T) {
	text, err := json.Marshal("**Retrieved 2 messages:** \n- (ID: 401) **[alice]** `2026-07-18T05:03:04.167Z`: ```hello```\n- (ID: 405) **[bot_test]** `2026-07-18T05:04:04.167Z`: ```bot reply```")
	require.NoError(t, err)
	caller := &fakeToolCaller{
		response: json.RawMessage(`{"content":[{"type":"text","text":` + string(text) + `}]}`),
	}
	adapter := NewDiscordAdapter(caller, "c1", "")
	adapter.SetBotUsername("bot_test")

	items, next, err := adapter.PollSince(context.Background(), Checkpoint{})

	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "401", items[0].ID)
	assert.Equal(t, "405", next.LastMessageID)
}

func TestDiscordAdapterIsResolvedDetectsOwnBotReplyReference(t *testing.T) {
	caller := &fakeToolCaller{
		response: json.RawMessage(`{"messages":[
			{"id":"201","content":"reply","author":{"id":"bot1","bot":true},"message_reference":{"message_id":"101"}}
		]}`),
	}
	adapter := NewDiscordAdapter(caller, "c1", "bot1")

	resolved, err := adapter.IsResolved(context.Background(), BoardItem{ID: "101", ChannelID: "c1"})

	require.NoError(t, err)
	assert.True(t, resolved)
	assert.Equal(t, "read_messages", caller.name)
	assert.Equal(t, "c1", caller.arguments["channelId"])
}

func TestDiscordAdapterIsResolvedIgnoresOtherReplies(t *testing.T) {
	caller := &fakeToolCaller{
		response: json.RawMessage(`{"messages":[
			{"id":"201","content":"other reply","author":{"id":"bot1","bot":true},"message_reference":{"message_id":"999"}},
			{"id":"202","content":"human reply","author":{"id":"u1","bot":false},"message_reference":{"message_id":"101"}}
		]}`),
	}
	adapter := NewDiscordAdapter(caller, "c1", "bot1")

	resolved, err := adapter.IsResolved(context.Background(), BoardItem{ID: "101", ChannelID: "c1"})

	require.NoError(t, err)
	assert.False(t, resolved)
}
