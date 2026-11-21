#!/bin/sh
# Fake "python" for service tests. Routes:
#   -m history_shorts validate --script S      → prints $FAKE_VALIDATE_OUT, exits $FAKE_VALIDATE_EXIT
#   -m history_shorts render ... --out O --preview → writes <dir O>/slides/01.png
#   tools/make_video.py --script S --work W --out O ... → prints make_video-like "$ " lines,
#        writes O and O's report; $FAKE_MV_SLEEP sleeps before the render step; exits $FAKE_MV_EXIT
out=""; work=""; preview=0
for a in "$@"; do
  case "$prev" in --out) out="$a";; --work) work="$a";; esac
  [ "$a" = "--preview" ] && preview=1
  prev="$a"
done
if [ "$1" = "-m" ] && [ "$3" = "validate" ]; then
  [ -n "$FAKE_VALIDATE_OUT" ] && printf '%s\n' "$FAKE_VALIDATE_OUT"
  echo "0 errors, 0 warnings"
  exit ${FAKE_VALIDATE_EXIT:-0}
fi
if [ "$1" = "-m" ] && [ "$3" = "render" ] && [ $preview = 1 ]; then
  d=$(dirname "$out"); mkdir -p "$d/slides"; echo png > "$d/slides/01.png"; echo png > "$d/slides/02.png"
  echo "Slides: 2"
  exit 0
fi
if [ "$1" = "tools/make_video.py" ]; then
  echo "\$ python -m history_shorts validate --script x"
  echo "0 errors, 0 warnings"
  echo "\$ python -m history_shorts export-narration --script x --out $work"
  mkdir -p "$work"
  for id in a b; do
    echo "\$ make-voice-free voice $work/$id.txt $work/$id.wav --voice p.json --allow-noncommercial"
    echo "$work/$id.wav: 4.0 s"
  done
  [ -n "$FAKE_MV_SLEEP" ] && sleep "$FAKE_MV_SLEEP"
  echo "\$ python -m history_shorts audio-manifest --narration $work"
  echo "\$ python -m history_shorts render --script x --out $out"
  if [ "${FAKE_MV_EXIT:-0}" != 0 ]; then echo "ERROR ffmpeg failed: boom"; exit $FAKE_MV_EXIT; fi
  mkdir -p "$(dirname "$out")"
  echo mp4 > "$out"
  echo '{"script_sha256":"x","total_duration":8.0,"slides":[{"index":1,"duration":4.0,"png":"slides/01.png"},{"index":2,"duration":4.0,"png":"slides/02.png"}]}' > "${out%.mp4}.report.json"
  echo "Slides: 2"
  exit 0
fi
echo "fake_python: unknown args $*" >&2
exit 9
