#!/bin/bash
# True Renkin Loop Command Runner (Codex)
set -o pipefail

AGENT_ID=${AGENT_ID:-default-agent}
echo "Starting True Renkin Loop Iteration for ${AGENT_ID}..."

RENKIN_LOOP_OUTPUT="$(mktemp)"
cleanup() {
  rm -f "$RENKIN_LOOP_OUTPUT"
}
trap cleanup EXIT

case '{llm_cmd}' in
  codex\ *)
    if [ ! -s /root/.codex/auth.json ] && [ -z "${OPENAI_API_KEY:-}" ]; then
      {
        echo "Codex authentication is not configured in the container."
        echo "Expected /root/.codex/auth.json, which is mounted from the host-side .renkin/codex/auth.json."
        echo "Run the auth helper in this agent directory:"
        echo "  renkin auth codex"
        echo "Or copy your host auth file outside the container:"
        echo "  cp ~/.codex/auth.json .renkin/codex/auth.json"
      } >&2
      exit 11
    fi
    ;;
esac

{llm_cmd} | tee "$RENKIN_LOOP_OUTPUT"
llm_status=${PIPESTATUS[0]}
if [ "$llm_status" -ne 0 ]; then
  echo "Renkin loop LLM command failed with exit code ${llm_status}." >&2
  exit "$llm_status"
fi

final_json="$(python3 - "$RENKIN_LOOP_OUTPUT" <<'PY'
import json
import sys

path = sys.argv[1]
last = ""
with open(path, "r", encoding="utf-8", errors="replace") as f:
    for line in f:
        text = line.strip()
        if not text:
            continue
        try:
            data = json.loads(text)
        except json.JSONDecodeError:
            continue
        if isinstance(data, dict) and "loop_status" in data:
            last = json.dumps(data, separators=(",", ":"))
print(last)
PY
)"

if [ -z "$final_json" ]; then
  echo 'Renkin loop final JSON contract missing. Expected a final JSON line with loop_status.' >&2
  exit 20
fi

echo "Renkin loop final JSON: $final_json"
loop_status="$(python3 - "$final_json" <<'PY'
import json
import sys
print(json.loads(sys.argv[1]).get("loop_status", ""))
PY
)"

case "$loop_status" in
  success|idle)
    exit 0
    ;;
  fatal)
    exit 10
    ;;
  *)
    echo "Unknown Renkin loop_status: $loop_status" >&2
    exit 21
    ;;
esac
