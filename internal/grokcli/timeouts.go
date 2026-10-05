package grokcli

import "time"

const (
	// SetupTimeout caps initialize + session/new. Real grok session/new
	// is ~700ms warm; cold starts may be slower.
	SetupTimeout = 20 * time.Second
	// PromptIdleTimeout is how long a session/prompt may sit with no
	// stdout activity before Rock aborts the wait (child stays alive).
	PromptIdleTimeout = 5 * time.Minute
)
