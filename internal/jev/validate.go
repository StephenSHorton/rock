package jev

import (
	"context"
	"fmt"
)

// Ping makes one real Decide call so a key is proven before Rock starts.
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || !c.Live() {
		return fmt.Errorf("jev key is not set")
	}
	_, err := c.Decide(ctx, map[string]string{"rock": "probe"}, map[string]Question{
		"ok": NoulQ("Reply that this request reached Jev."),
	})
	return err
}
