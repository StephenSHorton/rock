#!/usr/bin/env bash
# Record the five Rock feature demos in dark and light.
# See README.md in this directory.
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
VS="$ROOT/internal/tui/testdata/visual.sh"
export DISPLAY=${DISPLAY:-:1}
export ROCK_BIN=${ROCK_BIN:-/tmp/rock}
export ART=${ART:-/opt/cursor/artifacts/demos}
export WORK=${WORK:-/tmp/rock-demos}
export FONT_SIZE=${FONT_SIZE:-11}
export STUB_DELAY=${STUB_DELAY:-700ms}
SOCK=rock-visual
PANE=rock:0.0
DUR=${DUR:-10}

DARK_BG=#1E2228
DARK_FG=#ECEAE4
LIGHT_BG=#F7F5F0
LIGHT_FG=#1C1F24

mkdir -p "$ART" "$WORK/project"
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

geom() {
	xwininfo -id "$(xdotool search --name '^rock-visual$' | head -1)" | awk '
		/Absolute upper-left X/ {x=$4} /Absolute upper-left Y/ {y=$4}
		/Width:/ {w=$2} /Height:/ {h=$2} END {print x, y, w, h}'
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

# One ROCK_HOME reused for inspect, seeded with a real stub turn.
seed_inspect_home() {
	local home=$WORK/inspect-home
	if [ -d "$home" ]; then
		echo "$home"
		return
	fi
	mkdir -p "$home" "$WORK/project"
	printf 'helo wrold\n' >"$WORK/project/greeting.txt"
	cat >"$WORK/stub.toml" <<'EOF'
base_url = "http://127.0.0.1:18080/v1"
model = "stub-fast"
fast_model = "stub-fast"
strong_model = "stub-strong"
EOF
	ensure_llm
	env -u NO_COLOR -u OPENAI_API_KEY -u TYPESAFE_API_KEY \
		ROCK_HOME="$home" ROCK_API_KEY=stub ROCK_CONFIG="$WORK/stub.toml" \
		ROCK_TEST_FAKE_JEV=1 JEV_API_KEY=rock-test \
		COLORTERM=truecolor "$ROCK_BIN" --yolo --cwd "$WORK/project" \
		-p "fix the typo in greeting.txt" >/tmp/rock-inspect-seed.txt
	echo "$home"
}

open_tui() {
	local bg=$1 fg=$2
	kill_ui
	ensure_llm
	vs up 122 31 "$bg" "$fg"
	sleep 0.4
	vs stub
	wait_text 'Ask Rock' 50
	sleep 0.5
}

open_shell() {
	local bg=$1 fg=$2
	kill_ui
	vs up 122 31 "$bg" "$fg"
	sleep 0.4
}

type_enter() {
	t send-keys -t "$PANE" -l "$1"
	t send-keys -t "$PANE" Enter
}

record_raw() {
	local dest=$1
	local x y w h
	read -r x y w h < <(geom)
	# even dims for yuv420p
	w=$((w - w % 2))
	h=$((h - h % 2))
	ffmpeg -loglevel error -y -f x11grab -draw_mouse 0 -framerate 15 \
		-video_size "${w}x${h}" -i "$DISPLAY+$x,$y" -t "$DUR" \
		-c:v libx264 -pix_fmt yuv420p -preset fast -crf 28 \
		-movflags +faststart "$dest"
}

export_set() {
	local stem=$1 raw=$2
	local mp4=$ART/${stem}.mp4 gif=$ART/${stem}.gif png=$ART/${stem}.png
	ffmpeg -loglevel error -y -i "$raw" -c:v libx264 -pix_fmt yuv420p \
		-preset slow -crf 30 -movflags +faststart "$mp4"
	ffmpeg -loglevel error -y -i "$raw" -vf "fps=8,scale=iw:ih:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=48:stats_mode=diff[p];[s1][p]paletteuse=dither=bayer" "$gif"
	ffmpeg -loglevel error -y -sseof -0.15 -i "$raw" -frames:v 1 "$png"
	# If the gif is huge, shrink it; mp4 is the site player.
	local gsz
	gsz=$(stat -c%s "$gif")
	if [ "$gsz" -gt 2500000 ]; then
		ffmpeg -loglevel error -y -i "$raw" -vf "fps=6,scale=800:-2:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=32:stats_mode=diff[p];[s1][p]paletteuse=dither=bayer" "$gif"
	fi
	echo "$mp4"
}

drive_fork() {
	sleep 0.8
	type_enter "/fork look around"
	sleep 8
}

drive_plan() {
	sleep 0.6
	type_enter "/plan"
	wait_text 'plan mode' 20
	sleep 1.2
	type_enter "write a plan to fix the typo"
	wait_text 'Plan written' 40
	sleep 1.0
	type_enter "/ready"
	wait_text 'does not approve' 20
	sleep 5
}

drive_permission() {
	sleep 0.6
	type_enter "fix the typo in greeting.txt"
	wait_text 'Allow edit_file' 30
	sleep 2.2
	t send-keys -t "$PANE" y
	wait_text 'Edited' 40
	sleep 5
}

drive_agents() {
	sleep 0.6
	type_enter "use a subagent to explore greeting.txt"
	wait_text 'Allow spawn_subagent' 30
	sleep 1.0
	t send-keys -t "$PANE" y
	wait_text 'Subagent finished' 50
	sleep 0.8
	type_enter "/agents"
	wait_text 'Subagents' 20
	sleep 5
}

drive_inspect() {
	local home=$1
	sleep 0.5
	t send-keys -t "$PANE" -l "clear; ROCK_HOME=$home ROCK_API_KEY=stub ROCK_CONFIG=$WORK/stub.toml ROCK_TEST_FAKE_JEV=1 JEV_API_KEY=rock-test COLORTERM=truecolor $ROCK_BIN inspect --cwd $WORK/project"
	t send-keys -t "$PANE" Enter
	sleep 8
}

record_one() {
	local feat=$1 theme=$2
	local bg fg stem raw
	if [ "$theme" = light ]; then
		bg=$LIGHT_BG
		fg=$LIGHT_FG
	else
		bg=$DARK_BG
		fg=$DARK_FG
	fi
	stem="${feat}-${theme}"
	raw=$WORK/${stem}.mkv
	echo "== $stem =="
	case "$feat" in
	inspect)
		open_shell "$bg" "$fg"
		;;
	*)
		open_tui "$bg" "$fg"
		;;
	esac
	# ffmpeg -t DUR in the background; drive the UI while it records.
	record_raw "$raw" &
	local rec=$!
	sleep 0.2
	case "$feat" in
	fork) drive_fork ;;
	plan) drive_plan ;;
	permission) drive_permission ;;
	agents) drive_agents ;;
	inspect) drive_inspect "$(seed_inspect_home)" ;;
	*) echo "unknown feature $feat" >&2; return 2 ;;
	esac
	wait "$rec"
	export_set "$stem" "$raw"
}

probe_geom() {
	kill_ui
	vs up 122 31 "$DARK_BG" "$DARK_FG"
	sleep 0.4
	read -r _ _ w h < <(geom)
	echo "window ${w}x${h} (want ~1100x620)"
	# Keep going if we are in the neighborhood; FONT_SIZE is the knob.
}

main() {
	ensure_llm
	seed_inspect_home >/dev/null
	probe_geom
	local feats=(fork plan permission agents inspect)
	local themes=(dark light)
	if [ "$#" -gt 0 ]; then
		feats=("$@")
	fi
	local f th
	for th in "${themes[@]}"; do
		for f in "${feats[@]}"; do
			record_one "$f" "$th"
		done
	done
	kill_ui
	echo "exports in $ART"
	ls -la "$ART"
}

main "$@"
