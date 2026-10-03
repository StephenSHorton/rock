package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
)

func TestFramesAndTools(t *testing.T) {
	serverR, clientW := io.Pipe()
	clientR, serverW := io.Pipe()
	c := &Client{in: clientW, out: bufio.NewReader(clientR), pending: map[int]chan rpc{}}
	go c.readLoop()
	go func() {
		r := bufio.NewReader(serverR)
		for {
			body, err := readFrame(r)
			if err != nil {
				return
			}
			var msg rpc
			if json.Unmarshal(body, &msg) != nil {
				return
			}
			if msg.ID == nil {
				continue
			}
			var result any
			switch msg.Method {
			case "initialize":
				result = map[string]any{"protocolVersion": "2024-11-05"}
			case "tools/list":
				result = map[string]any{"tools": []map[string]any{{"name": "echo", "description": "echo", "inputSchema": map[string]any{"type": "object"}}}}
			case "tools/call":
				result = map[string]any{"content": []map[string]any{{"type": "text", "text": "pong"}}}
			default:
				result = map[string]any{}
			}
			raw, _ := json.Marshal(result)
			_ = writeFrame(serverW, rpc{JSONRPC: "2.0", ID: msg.ID, Result: raw})
		}
	}()
	ctx := context.Background()
	if _, err := c.call(ctx, "initialize", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	tools, err := c.Tools(ctx, "demo")
	if err != nil || len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("%v %#v", err, tools)
	}
	out, err := c.Call(ctx, "echo", map[string]any{"text": "hi"})
	if err != nil || out != "pong" {
		t.Fatalf("%v %q", err, out)
	}
}
