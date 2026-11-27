import hashlib
import json
from pathlib import Path
import re
import subprocess


root = Path('/tmp/budva-message-files-aQCDMb')
source = root / 'source'
target = source / 'internal/app/facade/file_download.go'
original = target.read_bytes()
original_hash = hashlib.sha256(original).hexdigest()
mutated, replacements = re.subn(rb'\bSynchronous:[ \t]*false\b', b'Synchronous: true', original)
if replacements != 1:
    raise RuntimeError('Expected exactly one async starter')
command = [
    'go', 'test', '-json', '-p', '1', '-count=1', '-timeout=60s',
    './internal/transport/grpc', '-run', '^TestListMessageFiles_RealBufconn',
]
evidence = {'source_sha256': original_hash, 'mutation': 'synchronous download starter', 'command': command}
try:
    target.write_bytes(mutated)
    red = subprocess.run(command, cwd=source, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=120)
    (root / 'selection-red.jsonl').write_bytes(red.stdout)
    evidence['red_exit'] = red.returncode
    output = red.stdout.decode(errors='replace')
    expected_tests = [
        'TestListMessageFiles_RealBufconnPhotoThenSelectNonDefaultDownload',
        'TestListMessageFiles_RealBufconnAlternativeVideoThenSelectDownload',
    ]
    if red.returncode == 0:
        raise RuntimeError('Synchronous selection mutant unexpectedly passed')
    for unexpected in ['[build failed]', '[setup failed]', 'panic:', 'test timed out', 'no tests to run']:
        if unexpected in output:
            raise RuntimeError('Unrelated failure is not causal evidence: ' + unexpected)
    events = [json.loads(line) for line in output.splitlines() if line.startswith('{')]
    for test_name in expected_tests:
        if not any(event.get('Test') == test_name and event.get('Action') == 'fail' for event in events):
            raise RuntimeError('Named selection test did not fail: ' + test_name)
        test_output = ''.join(event.get('Output', '') for event in events if event.get('Test') == test_name)
        if 'PROGRESSIVE_PAYLOAD_BEFORE_COMPLETION' not in test_output:
            raise RuntimeError('Expected causal diagnostic absent: ' + test_name)
    evidence['failed_tests'] = expected_tests
finally:
    target.write_bytes(original)
    if hashlib.sha256(target.read_bytes()).hexdigest() != original_hash:
        raise RuntimeError('Selection mutation restoration hash mismatch')
green = subprocess.run(command, cwd=source, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=120)
(root / 'selection-restored-green.jsonl').write_bytes(green.stdout)
evidence['restored_sha256'] = hashlib.sha256(target.read_bytes()).hexdigest()
evidence['restored_green_exit'] = green.returncode
(root / 'selection-mutation-evidence.json').write_text(json.dumps(evidence, indent=2) + '\n')
print(json.dumps(evidence, indent=2))
if green.returncode != 0:
    raise RuntimeError('Restored selection tests failed')
