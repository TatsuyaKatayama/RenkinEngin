package bot

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPClientCallToolInitializesAndSendsAcceptHeader(t *testing.T) {
	var requests []string
	var acceptHeaders []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptHeaders = append(acceptHeaders, r.Header.Get("Accept"))
		var req struct {
			Method string `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		requests = append(requests, req.Method)

		switch req.Method {
		case "initialize":
			w.Header().Set("Mcp-Session-Id", "session-1")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-03-26"}}`))
		case "notifications/initialized":
			w.WriteHeader(http.StatusAccepted)
		case "tools/call":
			assert.Equal(t, "session-1", r.Header.Get("Mcp-Session-Id"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":2,"result":{"ok":true}}`))
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer server.Close()

	client := NewMCPClient(server.URL, server.Client())
	result, err := client.CallTool(t.Context(), "read_messages", map[string]any{"channelId": "c1"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(result))
	assert.Equal(t, []string{"initialize", "notifications/initialized", "tools/call"}, requests)
	for _, accept := range acceptHeaders {
		assert.Equal(t, "text/event-stream, application/json", accept)
	}
}

func TestReadSSEData(t *testing.T) {
	data, err := readSSEData([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"result\":{\"ok\":true}}\n\n"))

	require.NoError(t, err)
	assert.JSONEq(t, `{"jsonrpc":"2.0","result":{"ok":true}}`, string(data))
}
