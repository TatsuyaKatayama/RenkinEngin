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
	assert.Equal(t, "101", next.LastMessageID)
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
