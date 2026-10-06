// Package mcp is a stdio MCP client. Frames use Content-Length, the same
// shape as LSP. Rock lists tools and calls them. It does not host a server.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/StephenSHorton/rock/internal/childproc"
	"github.com/StephenSHorton/rock/internal/config"
)

type Server struct {
	Name    string
	Command string
	Args    []string
}

type Tool struct {
	Server      string
	Name        string
	Description string
	InputSchema map[string]any
}

type Client struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	in      io.WriteCloser
	out     *bufio.Reader
	nextID  int
	pending map[int]chan rpc
	closed  bool
}

type rpc struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int            `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func Start(ctx context.Context, spec Server) (*Client, error) {
	cmd := exec.Command(spec.Command, spec.Args...)
	config.ScrubCmdEnv(cmd)
	childproc.Isolate(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	c := &Client{cmd: cmd, in: stdin, out: bufio.NewReader(stdout), pending: map[int]chan rpc{}}
	go c.readLoop()
	if _, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "rock", "version": "1.0.0"},
	}); err != nil {
		_ = c.Close()
		return nil, err
	}
	if err := c.notify("notifications/initialized", map[string]any{}); err != nil {
		_ = c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) Tools(ctx context.Context, server string) ([]Tool, error) {
	raw, err := c.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	out := make([]Tool, 0, len(parsed.Tools))
	for _, t := range parsed.Tools {
		out = append(out, Tool{Server: server, Name: t.Name, Description: t.Description, InputSchema: t.InputSchema})
	}
	return out, nil
}

func (c *Client) Call(ctx context.Context, name string, args map[string]any) (string, error) {
	raw, err := c.call(ctx, "tools/call", map[string]any{"name": name, "arguments": args})
	if err != nil {
		return "", err
	}
	var parsed struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return string(raw), nil
	}
	var b strings.Builder
	for _, part := range parsed.Content {
		b.WriteString(part.Text)
	}
	if parsed.IsError {
		return b.String(), fmt.Errorf("mcp tool error")
	}
	return b.String(), nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	if c.in != nil {
		_ = c.in.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
		_, _ = c.cmd.Process.Wait()
	}
	return nil
}

func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan rpc, 1)
	c.pending[id] = ch
	c.mu.Unlock()
	msg := rpc{JSONRPC: "2.0", Method: method}
	msg.ID = &id
	raw, _ := json.Marshal(params)
	msg.Params = raw
	if err := writeFrame(c.in, msg); err != nil {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		if res.Error != nil {
			return nil, fmt.Errorf("mcp %s: %s", method, res.Error.Message)
		}
		return res.Result, nil
	}
}

func (c *Client) notify(method string, params any) error {
	raw, _ := json.Marshal(params)
	return writeFrame(c.in, rpc{JSONRPC: "2.0", Method: method, Params: raw})
}

func (c *Client) readLoop() {
	for {
		body, err := readFrame(c.out)
		if err != nil {
			return
		}
		var msg rpc
		if json.Unmarshal(body, &msg) != nil || msg.ID == nil {
			continue
		}
		c.mu.Lock()
		ch := c.pending[*msg.ID]
		delete(c.pending, *msg.ID)
		c.mu.Unlock()
		if ch != nil {
			ch <- msg
		}
	}
}

func writeFrame(w io.Writer, msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(b), b)
	return err
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	length := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(strings.ToLower(line), "content-length:") {
			n, err := strconv.Atoi(strings.TrimSpace(line[len("content-length:"):]))
			if err != nil {
				return nil, err
			}
			length = n
		}
	}
	if length < 0 {
		return nil, fmt.Errorf("missing content-length")
	}
	buf := make([]byte, length)
	_, err := io.ReadFull(r, buf)
	return buf, err
}
