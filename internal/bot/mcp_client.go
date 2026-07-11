package bot

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

const mcpProtocolVersion = "2025-03-26"

type MCPClient struct {
	endpoint  string
	client    *http.Client
	sessionID string
	nextID    int
	mu        sync.Mutex
}

func NewMCPClient(endpoint string, client *http.Client) *MCPClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &MCPClient{
		endpoint: endpoint,
		client:   client,
		nextID:   1,
	}
}

func (c *MCPClient) CallTool(ctx context.Context, name string, arguments map[string]any) (json.RawMessage, error) {
	if err := c.ensureInitialized(ctx); err != nil {
		return nil, err
	}
	resp, err := c.rpc(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": arguments,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *MCPClient) ensureInitialized(ctx context.Context) error {
	c.mu.Lock()
	initialized := c.sessionID != ""
	c.mu.Unlock()
	if initialized {
		return nil
	}

	_, err := c.rpc(ctx, "initialize", map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo": map[string]any{
			"name":    "renkin-bot",
			"version": "0.1.0",
		},
	})
	if err != nil {
		return err
	}

	return c.notifyInitialized(ctx)
}

func (c *MCPClient) notifyInitialized(ctx context.Context) error {
	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, data)
	if err != nil {
		return err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("mcp initialized notification: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("mcp initialized notification status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *MCPClient) rpc(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	id := c.nextID
	c.nextID++
	c.mu.Unlock()

	reqBody := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := c.newRequest(ctx, data)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcp %s request: %w", method, err)
	}
	defer resp.Body.Close()

	if sessionID := resp.Header.Get("Mcp-Session-Id"); sessionID != "" {
		c.mu.Lock()
		c.sessionID = sessionID
		c.mu.Unlock()
	}

	body, err := readMCPBody(resp)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mcp %s status %d: %s", method, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &rpcResp); err != nil {
		return nil, fmt.Errorf("decode mcp %s response: %w", method, err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("mcp %s error %d: %s", method, rpcResp.Error.Code, rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

func (c *MCPClient) newRequest(ctx context.Context, data []byte) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream, application/json")

	c.mu.Lock()
	sessionID := c.sessionID
	c.mu.Unlock()
	if sessionID != "" {
		req.Header.Set("Mcp-Session-Id", sessionID)
	}
	return req, nil
}

func readMCPBody(resp *http.Response) ([]byte, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if strings.Contains(resp.Header.Get("Content-Type"), "text/event-stream") {
		return readSSEData(body)
	}
	return body, nil
}

func readSSEData(body []byte) ([]byte, error) {
	scanner := bufio.NewScanner(bytes.NewReader(body))
	var chunks []string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			chunks = append(chunks, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return nil, fmt.Errorf("mcp sse response contained no data event")
	}
	return []byte(strings.Join(chunks, "\n")), nil
}
