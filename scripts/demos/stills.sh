#!/usr/bin/env bash
# Capture hero TUI stills in dark and light. Same stub + fake Jev path as
# record.sh so the status line reads jev:live and the composer is Ask Rock.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
VS="$ROOT/internal/tui/testdata/visual.sh"
export DISPLAY=${DISPLAY:-:1}
export ROCK_BIN=${ROCK_BIN:-/tmp/rock}
export ART=${ART:-/opt/cursor/artifacts/tui}
export WORK=${WORK:-/tmp/rock-stills}
export FONT_SIZE=${FONT_SIZE:-12}
export STUB_DELAY=${STUB_DELAY:-700ms}
SOCK=rock-visual
PANE=rock:0.0

DARK_BG=#1E2228
DARK_FG=#ECEAE4
LIGHT_BG=#F7F5F0
LIGHT_FG=#1C1F24

mkdir -p "$ART" "$WORK/project" "$ART/text"
cd "$ROOT"
if [ ! -x "$ROCK_BIN" ]; then
	go build -o "$ROCK_BIN" ./cmd/rock
fi

t() { tmux -L "$SOCK" -f "$WORK/tmux.conf" "$@"; }
vs() { "$VS" "$@"; }

kill_ui() {
	for id in $(xdotool search --name '^rock-visual$' 2>/dev/null || true); do
		xkill -id "$id" 2>/dev/null || true
	done
	tmux -L "$SOCK" kill-server 2>/dev/null || true
}

wait_text() {
	local needle=$1 tries=${2:-40}
	local i out
	for i in $(seq "$tries"); do
		out=$(vs text 2>/dev/null || true)
		if echo "$out" | grep -q "$needle"; then
			return 0
		fi
		sleep 0.2
	done
	echo "timeout waiting for $needle" >&2
	echo "$out" >&2
	return 1
}

ensure_llm() {
	if ! tmux -L rock-llm has-session -t llm 2>/dev/null; then
		vs llm
		sleep 0.3
	fi
}

open_tui() {
	local bg=$1 fg=$2
	kill_ui
	ensure_llm
	# 140×36 @ 12pt lands near the 1400×792 hero frame (xterm chrome included).
	vs up 140 36 "$bg" "$fg"
	sleep 0.4
	vs stub
	wait_text 'Ask Rock' 50
	wait_text 'jev:live' 20
	if vs text | grep -q 'jev:offline'; then
		echo "refusing still: jev:offline on screen" >&2
		vs text >&2
		return 1
	fi
	if vs text | grep -q '/help /plan /yolo'; then
		echo "refusing still: old composer placeholder" >&2
		vs text >&2
		return 1
	fi
	sleep 0.4
}

type_enter() {
	t send-keys -t "$PANE" -l "$1"
	t send-keys -t "$PANE" Enter
}

dump() {
	local stem=$1
	vs text >"$ART/text/${stem}.txt"
	echo "--- $stem ---"
	grep -E 'jev:|Ask Rock|/ for commands|◇ jev|/help|/plan /yolo' "$ART/text/${stem}.txt" || true
}

shot() {
	local stem=$1
	ART="$ART" vs shot "$stem"
	dump "$stem"
}

capture_theme() {
	local theme=$1 bg fg
	if [ "$theme" = light ]; then
		bg=$LIGHT_BG
		fg=$LIGHT_FG
	else
		bg=$DARK_BG
		fg=$DARK_FG
	fi

	echo "== idle $theme =="
	open_tui "$bg" "$fg"
	sleep 0.3
	shot "idle-${theme}"

	echo "== slash $theme =="
	t send-keys -t "$PANE" -l "/"
	wait_text '/help' 20
	sleep 0.3
	shot "slash-${theme}"

	echo "== permission $theme =="
	open_tui "$bg" "$fg"
	type_enter "fix the typo in greeting.txt"
	wait_text 'Allow edit_file' 30
	sleep 0.6
	shot "permission-${theme}"

	echo "== tools $theme =="
	t send-keys -t "$PANE" y
	wait_text 'Edited' 40
	sleep 0.8
	shot "tools-${theme}"

	echo "== plan $theme =="
	open_tui "$bg" "$fg"
	type_enter "/plan"
	wait_text 'plan mode' 20
	sleep 0.4
	type_enter "write a plan to fix the typo"
	wait_text 'Plan written' 40
	sleep 0.8
	shot "plan-${theme}"

	echo "== askjev $theme =="
	open_tui "$bg" "$fg"
	type_enter "ask jev about rounding"
	wait_text '◇ jev' 40
	wait_text 'Jev answered' 40
	vs wheel up 4 || true
	sleep 0.5
	shot "askjev-${theme}"
}

main() {
	ensure_llm
	local themes=(dark light)
	if [ "$#" -gt 0 ]; then
		themes=("$@")
	fi
	local th
	for th in "${themes[@]}"; do
		capture_theme "$th"
	done
	kill_ui
	echo "stills in $ART"
	ls -la "$ART"
}

main "$@"
