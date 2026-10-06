// Package childproc keeps Rock's background subprocesses (the grok ACP
// child, MCP servers, shell tool calls, git checkpoints) off the user's
// console.
//
// On Windows a child spawned with plain exec.Command inherits Rock's
// console even when its stdio is piped. It can then open CONIN$/CONOUT$
// directly, read console input records (key presses and
// WINDOW_BUFFER_SIZE_EVENT resize records) that the fullscreen TUI needs,
// or reset the console mode when it exits. Isolate gives the child its own
// hidden console (CREATE_NO_WINDOW), the same thing Node's windowsHide does,
// so piped stdio keeps working and the TUI keeps its input. On other
// platforms Isolate does nothing.
package childproc

import "os/exec"

// Isolate detaches cmd from Rock's console where the platform needs it.
// Call it before cmd.Start. It keeps any SysProcAttr fields already set.
func Isolate(cmd *exec.Cmd) {
	if cmd == nil {
		return
	}
	isolate(cmd)
}
