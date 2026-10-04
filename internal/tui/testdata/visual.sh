#!/usr/bin/env bash
# Drive the Rock TUI on a real X display for visual checks. Rock runs in tmux
# inside xterm and keys go in through tmux send-keys: xdotool type into a
# fresh xterm leaks the terminal's bracketed-paste replies into the composer.
#
#   visual.sh up 140 36 [bg fg]   xterm + tmux at 140x36 (default: Rock panel)
#   visual.sh llm                 build and start stubllm on 127.0.0.1:18080
#   FONT_SIZE / STUB_DELAY        optional env for the xterm face and stub pause
#   visual.sh offline             rock with no model key (offline provider)
#   visual.sh stub                rock against stubllm in a fresh workspace
#   visual.sh askjev              stub + fake Jev; type "ask jev about rounding"
#   visual.sh onboard             first-run Jev gate (no saved key)
#   visual.sh onboard-ok          gate + ROCK_TEST_FAKE_JEV accept
#   visual.sh onboard-bad         gate + ROCK_TEST_FAKE_JEV reject
#   visual.sh onboard-slow        gate + fake accept after ROCK_TEST_FAKE_JEV_DELAY
#   visual.sh type "text"         type literally, then Enter
#   visual.sh keys Escape         tmux send-keys to the rock pane
#   visual.sh resize 80 30        resize the xterm, in cells
#   visual.sh wheel up|down [n]   mouse wheel over the transcript
#   visual.sh click COL ROW       left click on a cell (zero-based)
#   visual.sh shot NAME           PNG of the xterm into $ART
#   visual.sh text                the pane as plain text
#
# ROCK_BIN defaults to /tmp/rock (go build -o /tmp/rock ./cmd/rock).
set -euo pipefail

ROCK_BIN=${ROCK_BIN:-/tmp/rock}
ART=${ART:-/opt/cursor/artifacts}
WORK=${WORK:-/tmp/rock-visual}
FONT_SIZE=${FONT_SIZE:-12}
STUB_DELAY=${STUB_DELAY:-600ms}
SOCK=rock-visual
PANE=rock:0.0
TITLE=rock-visual
REPO=$(cd "$(dirname "$0")/../../.." && pwd)
export DISPLAY=${DISPLAY:-:1}

t() { tmux -L "$SOCK" -f "$WORK/tmux.conf" "$@"; }
wid() { xdotool search --name "^$TITLE\$" | head -1; }

geom() {
	xwininfo -id "$(wid)" | awk '
		/Absolute upper-left X/ {x=$4} /Absolute upper-left Y/ {y=$4}
		/Width:/ {w=$2} /Height:/ {h=$2} END {print x, y, w, h}'
}

cell() {
	local w h cols rows
	read -r _ _ w h < <(geom)
	cols=$(t display -p -t "$PANE" '#{pane_width}')
	rows=$(t display -p -t "$PANE" '#{pane_height}')
	echo $((w / cols)) $((h / rows))
}

up() {
	local cols=${1:-140} rows=${2:-36} bg=${3:-#1E2228} fg=${4:-#ECEAE4}
	mkdir -p "$WORK/project" "$ART"
	cat >"$WORK/tmux.conf" <<-'EOF'
		set -g default-terminal "tmux-256color"
		set -as terminal-features ",xterm*:RGB"
		set -as terminal-overrides ",xterm*:Tc"
		set -g status off
		set -g escape-time 10
		set -g mouse off
	EOF
	t kill-server 2>/dev/null || true
	env -u NO_COLOR tmux -L "$SOCK" -f "$WORK/tmux.conf" new-session -d -s rock -x "$cols" -y "$rows" -c "$WORK/project"
	xterm -b 0 -geometry "${cols}x${rows}+16+16" -fa 'JetBrains Mono' -fs "$FONT_SIZE" \
		-bg "$bg" -fg "$fg" -cr '#7A9BFF' -T "$TITLE" -n "$TITLE" \
		-e tmux -L "$SOCK" -f "$WORK/tmux.conf" attach -t rock >/dev/null 2>&1 &
	for _ in $(seq 50); do
		[ -n "$(wid)" ] && break
		sleep 0.1
	done
	wid
}

llm() {
	(cd "$REPO" && go build -o "$WORK/stubllm" ./internal/tui/testdata/stubllm)
	tmux -L rock-llm has-session -t llm 2>/dev/null ||
		tmux -L rock-llm new-session -d -s llm "$WORK/stubllm -addr 127.0.0.1:18080 -delay $STUB_DELAY 2>&1 | tee $WORK/stubllm.log"
}

# run UNSETS ASSIGNMENTS starts rock in the pane with a fresh ROCK_HOME.
# NO_COLOR is dropped on purpose: Rock honors it, and this is a color check.
run() {
	local home
	home=$(mktemp -d /tmp/rock-home.XXXXXX)
	t send-keys -t "$PANE" "clear; cd $WORK/project && env -u NO_COLOR -u OPENAI_API_KEY -u JEV_API_KEY -u TYPESAFE_API_KEY -u ROCK_TEST_FAKE_JEV -u ROCK_TEST_FAKE_JEV_DELAY $1 ROCK_HOME=$home COLORTERM=truecolor $2 $ROCK_BIN" Enter
}

offline() {
	run "-u ROCK_API_KEY" "ROCK_TEST_FAKE_JEV=1 JEV_API_KEY=rock-test ROCK_CONFIG=$WORK/offline-none.toml"
}

stub() {
	printf 'helo wrold\n' >"$WORK/project/greeting.txt"
	cat >"$WORK/stub.toml" <<-'EOF'
		base_url = "http://127.0.0.1:18080/v1"
		model = "stub-fast"
		fast_model = "stub-fast"
		strong_model = "stub-strong"
	EOF
	run "" "ROCK_TEST_FAKE_JEV=1 JEV_API_KEY=rock-test ROCK_API_KEY=stub ROCK_CONFIG=$WORK/stub.toml"
}

# onboard starts Rock with no Jev key so the blocking gate is the first screen.
# Extra env (fake accept/reject/delay) is appended after the unsets.
onboard() {
	run "-u ROCK_API_KEY" "$*"
}

wheel() {
	local cw ch button=5
	read -r cw ch < <(cell)
	[ "$1" = up ] && button=4
	xdotool mousemove --window "$(wid)" $((cw * 20)) $((ch * 6)) click --repeat "${2:-3}" --delay 80 "$button"
}

click() {
	local cw ch
	read -r cw ch < <(cell)
	xdotool mousemove --window "$(wid)" $((cw * $1 + cw / 2)) $((ch * $2 + ch / 2)) click 1
}

shot() {
	local x y w h
	read -r x y w h < <(geom)
	ffmpeg -loglevel error -y -f x11grab -draw_mouse 0 -video_size "${w}x${h}" -i "$DISPLAY+$x,$y" -frames:v 1 "$ART/$1.png"
	echo "$ART/$1.png"
}

cmd=${1:-}
shift || true
case "$cmd" in
up) up "$@" ;;
llm) llm ;;
offline) offline ;;
stub) stub ;;
askjev) stub ;;
onboard) onboard "$@" ;;
onboard-ok) onboard "ROCK_TEST_FAKE_JEV=1" ;;
onboard-bad) onboard "ROCK_TEST_FAKE_JEV=reject" ;;
onboard-slow) onboard "ROCK_TEST_FAKE_JEV=1 ROCK_TEST_FAKE_JEV_DELAY=${1:-8s}" ;;
type) t send-keys -t "$PANE" -l "$1" && t send-keys -t "$PANE" Enter ;;
keys) t send-keys -t "$PANE" "$@" ;;
resize) xdotool windowsize --usehints "$(wid)" "$1" "$2" ;;
wheel) wheel "$@" ;;
click) click "$@" ;;
shot) shot "$1" ;;
text) t capture-pane -p -t "$PANE" ;;
*) sed -n '2,24p' "$0" | sed 's/^# \{0,1\}//' && exit 2 ;;
esac
