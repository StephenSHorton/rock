package provider

import (
	"context"
	"fmt"
)

// Script is a deterministic provider. Each Complete call pops the next
// message. When the queue is empty it returns a short assistant reply.
type Script struct {
	Replies []Message
	Calls   int
}

func (s *Script) Name() string { return "offline" }

func (s *Script) Complete(_ context.Context, _ string, _ []Message, _ []ToolSpec) (Message, error) {
	s.Calls++
	if len(s.Replies) == 0 {
		return Message{Role: RoleAssistant, Content: "Rock is up. No model key is set, so this reply is the offline provider."}, nil
	}
	msg := s.Replies[0]
	s.Replies = s.Replies[1:]
	if msg.Role == "" {
		msg.Role = RoleAssistant
	}
	return msg, nil
}

func (s *Script) String() string { return fmt.Sprintf("offline calls=%d", s.Calls) }
