package provider

import "context"

// StreamKind tags a live delta from a provider.
type StreamKind string

const (
	// StreamText is reply text as it arrives. The final Message is still
	// authoritative (tool fences are parsed from the final text).
	StreamText StreamKind = "text"
	// StreamThought is model reasoning text (grok agent_thought_chunk).
	StreamThought StreamKind = "thought"
)

// StreamFunc receives deltas while Complete runs. It must not block for
// long; it is called on the provider's reader goroutine.
type StreamFunc func(kind StreamKind, delta string)

type streamKey struct{}

// WithStream asks providers that can stream to report deltas to f while
// Complete runs on the returned context. Providers that cannot stream
// ignore it and only return the final Message.
func WithStream(ctx context.Context, f StreamFunc) context.Context {
	if f == nil {
		return ctx
	}
	return context.WithValue(ctx, streamKey{}, f)
}

// StreamFrom returns the stream callback on ctx, or nil.
func StreamFrom(ctx context.Context) StreamFunc {
	if ctx == nil {
		return nil
	}
	f, _ := ctx.Value(streamKey{}).(StreamFunc)
	return f
}
