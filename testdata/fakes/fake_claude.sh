#!/bin/sh
# Fake Claude CLI for tests.
# FAKE_CLAUDE_STATE: counter file; FAKE_CLAUDE_REPLY_<n>: file whose content is printed on call n
# (falls back to FAKE_CLAUDE_REPLY); FAKE_CLAUDE_LOG: stdin of call n is saved to <log>.<n>;
# FAKE_CLAUDE_EXIT: exit code (default 0); FAKE_CLAUDE_SLEEP: seconds to sleep first.
n=1
if [ -n "$FAKE_CLAUDE_STATE" ]; then
  if [ -f "$FAKE_CLAUDE_STATE" ]; then n=$(( $(cat "$FAKE_CLAUDE_STATE") + 1 )); fi
  echo "$n" > "$FAKE_CLAUDE_STATE"
fi
if [ -n "$FAKE_CLAUDE_LOG" ]; then cat > "$FAKE_CLAUDE_LOG.$n"; else cat > /dev/null; fi
if [ -n "$FAKE_CLAUDE_SLEEP" ]; then sleep "$FAKE_CLAUDE_SLEEP"; fi
eval reply=\${FAKE_CLAUDE_REPLY_$n:-\$FAKE_CLAUDE_REPLY}
if [ -n "$reply" ]; then cat "$reply"; fi
exit ${FAKE_CLAUDE_EXIT:-0}
