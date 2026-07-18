package bot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookNotifierPostsExhaustedPayload(t *testing.T) {
	var payload WebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	notifier := WebhookNotifier{URL: server.URL, Client: server.Client()}
	err := notifier.NotifyExhausted(t.Context(), DispatchRecord{
		BoardItemID: "101",
		Attempt:     2,
		MaxRetries:  2,
		State:       DispatchStateExhausted,
		BoardItem:   BoardItem{ID: "101", ChannelID: "c1"},
	})

	require.NoError(t, err)
	assert.Equal(t, "renkin.bot.dispatch.exhausted", payload.Event)
	assert.Equal(t, "101", payload.BoardItemID)
	assert.Equal(t, 2, payload.Attempt)
	assert.Equal(t, DispatchStateExhausted, payload.State)
	assert.Equal(t, "c1", payload.BoardItem.ChannelID)
	assert.False(t, payload.Timestamp.IsZero())
}

func TestWebhookNotifierReturnsStatusError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := WebhookNotifier{URL: server.URL, Client: server.Client()}
	err := notifier.NotifyExhausted(t.Context(), DispatchRecord{BoardItemID: "101"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "webhook status 500")
}
