/** Captured from `go build -o rock ./cmd/rock` on this checkout, then run offline. */

export const helpText = `rock is a coding agent.

  rock                         fullscreen TUI
  rock -p "prompt"             one headless turn, text on stdout
  rock -p "prompt" --output streaming-json
  rock --resume ID             open a session in the TUI
  rock --session-id ID -- prompt
  rock --yolo -p "prompt"      skip asks; destructive commands still block
  rock --mode plan -p "prompt"

  rock inspect                 config, skills, MCP, Jev mode
  rock setup                   Huh form; does not write API keys
  rock sessions                list sessions for this folder
  rock permissions             allow / ask / deny
  rock fork                    print a Suzuri OSC 7880 sequence
  rock serve                   loopback HTTP and SSE
  rock acp                     ACP v1 and v2 on stdio
  rock update                  install the latest GitHub release
  rock update --check          print current -> latest
  rock version

Sessions live under ~/.rock (ROCK_HOME). Config is ~/.config/rock/config.toml (ROCK_CONFIG).
Model keys: ROCK_API_KEY or OPENAI_API_KEY. Jev keys: JEV_API_KEY or TYPESAFE_API_KEY.`

export const inspectText = `rock 1.0.0
cwd: /workspace
config: ~/.config/rock/config.toml
mode: default
review_only: false
trusted: false
provider: offline
model key: unset (ROCK_API_KEY or OPENAI_API_KEY). Replies come from the offline provider.
jev: offline
jev key: unset (JEV_API_KEY or TYPESAFE_API_KEY). Gates run a local policy. That policy is not Jev.
git.checkpoint: false
allow: read_file, grep, glob, web_fetch, jev_decide, update_plan
ask: edit_file, write_file, shell, spawn_subagent
deny: 
skills: none
mcp: none configured
sessions: ~/.rock
Permissions are not a sandbox. Plan mode blocks every shell command because redirections are not inspected.
Suzuri: rock fork prints OSC 7880. The host allowlist must include the rock binary.`

export const printStdout = `Rock is up. No model key is set, so this reply is the offline provider.`

export const printStderr = `jev turn offline model=fast stuck=false compact=false skills= (offline policy)
done end_turn`

export const jsonStdout = `{"kind":"jev","name":"turn","text":"offline model=fast stuck=false compact=false skills= (offline policy)"}
{"kind":"assistant","text":"Rock is up. No model key is set, so this reply is the offline provider."}
{"kind":"done","text":"end_turn"}`

export const permissionsText = `mode default
allow read_file
allow grep
allow glob
allow web_fetch
allow jev_decide
allow update_plan
ask   edit_file
ask   write_file
ask   shell
ask   spawn_subagent
Permissions are not a sandbox. Plan mode blocks every shell command because redirections are not inspected.`

export const tuiTranscript = ` rock  Does this comment match the test? · a18f3c0e9b21d4c6          ~/src/app
 DEFAULT  jev:live  ◆ gpt-4o-mini (offline)  ctx ░░░░░░   2%            ready

you   Does this comment match the test?
      ◇ jev turn  offline model=fast stuck=false compact=false skills= (offline policy)
rock  Rock is up. No model key is set, so this reply is the offline provider.

 Ask Rock                                          / for commands`

export const commands = [
  { cmd: 'rock', detail: 'Fullscreen Charm TUI. Transcript, composer, plan pane, permission modal.' },
  { cmd: 'rock -p "…"', detail: 'One headless turn. Assistant text on stdout; tool and Jev lines on stderr.' },
  { cmd: 'rock -p "…" --output streaming-json', detail: 'One JSON event per line on stdout: jev, assistant, tool_call, permission, done.' },
  { cmd: 'rock --resume ID', detail: 'Open an existing session in the TUI.' },
  { cmd: 'rock --yolo -p "…"', detail: 'Skip asks. The destructive gate still blocks.' },
  { cmd: 'rock --mode plan -p "…"', detail: 'Plan mode. Edits and every shell command are blocked.' },
  { cmd: 'rock acp', detail: 'ACP v1 and v2 on stdio. Editors speak JSON-RPC to the same harness.' },
  { cmd: 'rock serve', detail: 'Loopback HTTP and SSE. Default 127.0.0.1:8787.' },
  { cmd: 'rock inspect', detail: 'Config, skills, MCP, Jev key source, permissions.' },
  { cmd: 'rock setup', detail: 'Huh form. Writes ~/.config/rock/config.toml. Does not store API keys.' },
  { cmd: 'rock setup jev', detail: 'Validate a Jev key and save it. Required before the agent starts.' },
  { cmd: 'rock login chatgpt', detail: 'Sign in with ChatGPT (OSS SIWC). Model credentials only. Not Jev.' },
  { cmd: 'rock fork', detail: 'Prints OSC 7880 so Suzuri can split a pane.' },
  { cmd: 'rock update', detail: 'Install the latest GitHub release. Checksum, then atomic replace. go install trees get the module command.' },
  { cmd: 'rock update --check', detail: 'Print current -> latest without installing.' },
  { cmd: 'rock version', detail: 'Prints the build version (dev unless a tagged release injected one).' },
  { cmd: 'rock sessions', detail: 'List sessions for this folder.' },
  { cmd: 'rock permissions', detail: 'allow / ask / deny. Permissions are not a sandbox.' },
]

export const slash = [
  { cmd: '/help', detail: 'this help' },
  { cmd: '/plan', detail: 'plan mode: edits and shell blocked' },
  { cmd: '/yolo', detail: 'skip asks; destructive gate stays' },
  { cmd: '/default', detail: 'back to the default policy' },
  { cmd: '/sessions', detail: 'resume a session from this folder' },
  { cmd: '/permissions', detail: 'allow, ask, and deny rules' },
  { cmd: '/agents', detail: 'subagents spawned this session' },
  { cmd: '/ready', detail: 'is the plan ready? never approves' },
  { cmd: '/fork', detail: 'new Rock pane in Suzuri (OSC 7880)' },
  { cmd: '/update', detail: 'install the latest Rock release' },
  { cmd: '/quit', detail: 'quit' },
]
