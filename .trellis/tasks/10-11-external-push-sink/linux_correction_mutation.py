"""Prove correction regressions red in an isolated source copy; always restore."""
import hashlib
import json
from pathlib import Path
import subprocess

root = Path(__file__).resolve().parent
path = root / "source/internal/app/handler/service.go"
original = path.read_bytes()
source_routes = b"""for _, rule := range rs.ForwardRules {
			if rule.From == msg.ChatId {
				names = append(names, rule.PushTo...)
			}
		}"""
copy_routes = b"""for _, copy := range copies {
			if ref, ok := parseCopyRef(copy); ok {
				if rule := rs.ForwardRules[ref.ruleID]; rule != nil {
					names = append(names, rule.PushTo...)
				}
			}
		}"""
cases = [
    ("copy_guard", b"if len(copies) == 0 && (emission == nil || emission.push == nil) {",
     b"if len(copies) == 0 {", "TestPush_EditWithoutCopies"),
    ("copy_routes", source_routes, copy_routes,
     "TestPush_SourceEditRoutingIndependentOfCopies/mapped_and_unmapped_rules"),
    ("album_hook", b"s.processMediaAlbum(ctx, albumKey, rs.RuleSet, ruleID, msg, formattedText, rs.push)",
     b"s.processMediaAlbum(ctx, albumKey, rs.RuleSet, ruleID, msg, formattedText)",
     "TestPush_AlbumPipelineInvariant/forward/success"),
    ("album_timestamp", b"OccurredAt: occurredAt, Text: text.Text,",
     b"OccurredAt: time.Now().UTC(), Text: text.Text,",
     "TestPush_AlbumNewDoesNotSupersedeEditsOrDeletes"),
    ("album_wait", b"case <-time.After(3*time.Second - age):",
     b"case <-time.After(0):", "TestPush_AlbumPipelineInvariant/forward/success"),
]
evidence = []
for name, old, new, test in cases:
    assert original.count(old) == 1, (name, "ambiguous mutation")
    mutated = original.replace(old, new, 1)
    # Keep the timestamp variable referenced so the mutation tests behavior,
    # not Go's unused-variable compiler check.
    if name == "album_timestamp":
        mutated = mutated.replace(b"occurredAt := time.Now().UTC()",
                                  b"occurredAt := time.Now().UTC(); _ = occurredAt", 1)
    try:
        path.write_bytes(mutated)
        result = subprocess.run(
            ["sh", str(root / "linux_correction_validate.sh"), "test", "-tags", "libtdjson",
             "-p", "2", "-count=1", "./internal/app/handler", "-run", "^" + test + "$"],
            capture_output=True, text=True, timeout=300,
        )
        output = result.stdout + result.stderr
        (root / (name + "-red.log")).write_text(output)
        assert result.returncode != 0, name + " mutation unexpectedly green"
        assert "[build failed]" not in output, name + " is a compilation failure"
        assert "--- FAIL:" in output, name + " has no behavioral test failure"
        evidence.append({"case": name, "test": test, "exit": result.returncode})
        print(name + " RED verified", flush=True)
    finally:
        path.write_bytes(original)

assert path.read_bytes() == original
(root / "mutation-results.json").write_text(json.dumps({
    "cases": evidence, "restored_sha256": hashlib.sha256(original).hexdigest(),
}, indent=2) + "\n")
