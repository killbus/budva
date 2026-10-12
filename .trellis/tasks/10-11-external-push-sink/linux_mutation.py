"""Prove new regression tests red in the isolated source copy, then restore."""
from pathlib import Path
import subprocess

root = Path('/home/ying/budva-push-validation-20261011')
cases = [
    ('hooks', 'internal/app/handler/service.go',
     'push: submitter}', 'push: nil}',
     './internal/app/handler', 'TestPush'),
    ('validation', 'internal/infra/ruleset/repo.go',
     'func validate(rs *domain.RuleSet) error {',
     'func validate(rs *domain.RuleSet) error { return nil; ',
     './internal/infra/ruleset', 'TestPushTarget'),
    ('retirement', 'internal/service/push/manager.go',
     'previous.Retire()', '_ = previous',
     './internal/service/push', 'TestManager_ConcurrentSubmitReloadAndClose'),
    ('redirect_body', 'internal/service/push/webhook.go',
     'for name, value := range s.headers {',
     'req.GetBody = nil; for name, value := range s.headers {',
     './internal/service/push', 'TestWebhookSink_PreservesBodyAcrossHTTPSRedirect'),
]
for name, filename, old, new, package, test in cases:
    path = root / 'source' / filename
    original = path.read_bytes()
    assert old.encode() in original
    try:
        path.write_bytes(original.replace(old.encode(), new.encode(), 1))
        result = subprocess.run(['sh', str(root / 'linux_validate.sh'), 'test',
                                 '-tags', 'libtdjson', package, '-run', test,
                                 '-count=1'], capture_output=True, text=True)
        (root / (name + '-red.log')).write_text(result.stdout + result.stderr)
        assert result.returncode != 0, name + ' mutation unexpectedly green'
        assert '[build failed]' not in result.stdout + result.stderr
        print(name, 'RED verified', flush=True)
    finally:
        path.write_bytes(original)
