#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root"
source_file=internal/app/facade/file_download.go
proof_dir=$(mktemp -d)
cp "$source_file" "$proof_dir/original.go"
restore_source() {
  cp "$proof_dir/original.go" "$source_file"
}
trap restore_source EXIT

run_progressive() {
  go test -json -count=1 -timeout=90s ./internal/transport/grpc -run '^TestProgressiveDownloadBeforeCompletion$'
}

echo 'Progressive download: original source must pass'
run_progressive | tee "$proof_dir/green-before.jsonl"
python3 - "$proof_dir/green-before.jsonl" <<'PY'
import json
import sys

events = [json.loads(line) for line in open(sys.argv[1], encoding="utf-8")]
assert any(event.get("Action") == "pass" and event.get("Test") == "TestProgressiveDownloadBeforeCompletion" for event in events), "progressive test did not run and pass"
PY

python3 - "$source_file" <<'PY'
from pathlib import Path
import re
import sys

source = Path(sys.argv[1])
original = source.read_bytes()
mutated, count = re.subn(rb"\bSynchronous:[ \t]*false\b", b"Synchronous: true", original)
if count != 1:
    raise SystemExit(f"expected one async download starter, found {count}")
source.write_bytes(mutated)
PY

echo 'Progressive download: synchronous starter must fail before the completion barrier opens'
set +e
run_progressive | tee "$proof_dir/red.jsonl"
red_status=${PIPESTATUS[0]}
set -e
restore_source
cmp -s "$source_file" "$proof_dir/original.go"
if [[ "$red_status" -eq 0 ]]; then
  echo 'Synchronous mutant unexpectedly passed' >&2
  exit 1
fi
python3 - "$proof_dir/red.jsonl" <<'PY'
import json
import sys

events = [json.loads(line) for line in open(sys.argv[1], encoding="utf-8")]
test_name = "TestProgressiveDownloadBeforeCompletion"
assert any(event.get("Action") == "fail" and event.get("Test") == test_name for event in events), "mutant did not fail the named progressive test"
output = "".join(event.get("Output", "") for event in events)
assert "PROGRESSIVE_PAYLOAD_BEFORE_COMPLETION" in output, "missing intended payload-before-completion failure"
for unrelated_failure in ("[build failed]", "[setup failed]", "panic:", "test timed out", "no tests to run"):
    assert unrelated_failure not in output, f"unrelated failure cannot prove red: {unrelated_failure}"
assert not any(event.get("Action") == "fail" and event.get("Test") and not event["Test"].startswith(test_name) for event in events), "unrelated test failed"
PY

echo 'Progressive download: restored source must pass again'
run_progressive | tee "$proof_dir/green-after.jsonl"
cmp -s "$source_file" "$proof_dir/original.go"
python3 - "$proof_dir/green-after.jsonl" <<'PY'
import json
import sys

events = [json.loads(line) for line in open(sys.argv[1], encoding="utf-8")]
assert any(event.get("Action") == "pass" and event.get("Test") == "TestProgressiveDownloadBeforeCompletion" for event in events), "restored progressive test did not pass"
PY
printf 'Progressive red/green verified; temporary evidence: %s\n' "$proof_dir"
