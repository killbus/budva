import hashlib
import json
from pathlib import Path
import subprocess


root = Path('/tmp/budva-message-files-aQCDMb')
source = root / 'source'
target = source / 'internal/app/facade/message_files.go'
original = target.read_bytes()
original_hash = hashlib.sha256(original).hexdigest()
anchor = b'\tresult.Items, result.ContentSupported = messageMediaItems(msg.Content)'
if original.count(anchor) != 1:
    raise RuntimeError('Enumeration mutation anchor is not unique')
command = [
    'go', 'test', '-p', '1', '-count=1', '-timeout=60s',
    './internal/app/facade',
    '-run', '^TestService_ListMessageFiles_(PhotoMutationBindsExtraction|PhotoSizesMetadataOnly)$',
]
evidence = {'source_sha256': original_hash, 'mutation': 'omit enumerated items after real extraction', 'command': command}
try:
    target.write_bytes(original.replace(anchor, anchor + b'\n\tresult.Items = nil'))
    red = subprocess.run(command, cwd=source, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=120)
    (root / 'enumeration-red.log').write_bytes(red.stdout)
    evidence['red_exit'] = red.returncode
    red_text = red.stdout.decode(errors='replace')
    expected_tests = ['TestService_ListMessageFiles_PhotoMutationBindsExtraction', 'TestService_ListMessageFiles_PhotoSizesMetadataOnly']
    if red.returncode == 0 or '[build failed]' in red_text or 'panic:' in red_text:
        raise RuntimeError('Mutation did not yield the required assertion failure')
    for test_name in expected_tests:
        if '--- FAIL: ' + test_name not in red_text:
            raise RuntimeError('Expected assertion test did not fail: ' + test_name)
    evidence['failed_tests'] = expected_tests
finally:
    target.write_bytes(original)
    if hashlib.sha256(target.read_bytes()).hexdigest() != original_hash:
        raise RuntimeError('Mutation restoration hash mismatch')
green = subprocess.run(command, cwd=source, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=120)
(root / 'enumeration-restored-green.log').write_bytes(green.stdout)
evidence['restored_sha256'] = hashlib.sha256(target.read_bytes()).hexdigest()
evidence['restored_green_exit'] = green.returncode
(root / 'enumeration-mutation-evidence.json').write_text(json.dumps(evidence, indent=2) + '\n')
print(json.dumps(evidence, indent=2))
if green.returncode != 0:
    raise RuntimeError('Restored enumeration tests failed; see enumeration-restored-green.log')
