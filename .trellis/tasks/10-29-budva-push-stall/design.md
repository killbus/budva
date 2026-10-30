# Design: producer-side signal map (source-verified)

> Status (2026-10-30): the 10-29 diagnosis is complete — root cause,
> evidence, and fix: `research/root-cause.md`. This file is the standing
> reference for the push pipeline's **observability surface** — where it
> logs, where it counts, where it is silent — verified against `main` @
> `d8830f0`. The fix did not touch any of these signal points (it changed
> only the queue lock boundary in `internal/infra/queue/repo.go`).

## Topology (producer side)

- Producer: the budva engine. `ruleset.yml`: source chats; one configured
  push target — an HTTPS endpoint, a black box from the producer's vantage
  (accepted / rejected (class) / unreachable); webhook-only forward rules
  (`pushTo` only, no Telegram `to`).
- Deployed shape (2026-10-29): 17 source chats; the target configured with
  `schemaVersion: 2`, `timeoutSeconds: 60`, a static Basic-auth header, and
  an `hmacSecret`.

## Pipeline with the verified signal points

```
TDLib client
  → handler (internal/app/handler/service.go)
      rule match: rule.From == msg.ChatId
      eligibility gates (ALL SILENT — no log, no metric):
        - msg.Content != nil            (service.go:255; system/service-type
                                         events carry no content, dropped)
        - v2SnapshotFor(...) != nil     (SubmitV2Only paths, service.go:258)
        - albums: MediaAlbumId != 0 → quiet-period collection path
        - edits: separate collection, all matching rules' pushTo names
          merged into ONE SubmitV2Only (service.go:758-764)
  → dispatcher Submit (internal/service/push/dispatcher.go)
      lease not acquirable (retired generation):
                                     → SubmitRejectedRetired + rejected_retired_total
                                       + WARN "Push submission rejected"
                                       (reason=retired)  (dispatcher.go:128-138)
      endpoint unknown to the generation:
                                     → SubmitRejectedUnknown — SILENT: no log,
                                       no metric  (dispatcher.go:151-155)
      select per known endpoint (non-blocking):
        worker.queue <- rend            → SubmitAccepted
                                             + budva.push.enqueued_total
                                             (dispatcher.go:157-160)
        default:                        → SubmitRejectedBufferFull
                                             + budva.push.rejected_buffer_full_total
                                             + WARN "Push submission rejected"
                                             (reason=buffer_full)  (dispatcher.go:161-164)
  → endpointWorker goroutine (one per endpoint, serial) (dispatcher.go:214-237)
      ctx = WithTimeout(Background, worker.timeout)   // timeoutSeconds
      err := sink.Push(ctx, rend)
        failure → WARN "Push delivery failed" (endpoint, class)
                  (dispatcher.go:229)
                  + budva.push.delivery_failed_total{class}
                  + budva.push.send_duration_ms (recorded on every attempt)
        success → lastSuccess[endpoint] = now (dispatcher.go:230-234)
  → WebhookSink.Push (internal/service/push/delivery.go)
      url must parse + scheme https + host else FailureTLSTransport
      Content-Type: application/json; static headers set;
      hmacSecret != "" → X-Budva-Signature: sha256=<hex> over EXACT body
      singleAttemptTransport: NO retries on the POST; 307/308 redirect
      following stays inside the same attempt
      non-2xx → DeliveryError class (see classifyDeliveryError)
```

## Observable signals

Metrics (OTel meter `github.com/pure-golang/budva-claude/internal/service/
push`, instrument names, `internal/service/push/metrics.go:34-56`):

| metric | meaning |
| --- | --- |
| `budva.push.enqueued_total{endpoint}` | accepted into the delivery queue |
| `budva.push.rejected_buffer_full_total{endpoint}` | queue full, dropped |
| `budva.push.rejected_retired_total{endpoint}` | rejected to a retired generation |
| `budva.push.delivery_failed_total{endpoint, class}` | Push failures by class |
| `budva.push.reload_total{outcome}` | loadSnapshot outcomes (ok/failed) |
| `budva.push.send_duration_ms` | per-attempt duration histogram |
| `budva.push.buffer_depth{endpoint}` | current queue depth gauge |
| `budva.push.last_success_unix{endpoint}` | unix ts of last successful delivery |

Whether they are exposed to an HTTP endpoint depends on the monitoring
initialization; in this deployment the metrics backend is not configured, so
none are exported.

Logs (slog):
- `INFO  Push generation reloaded` (added/removed) — `internal/service/push/manager.go` (Replace)
- `INFO  Ruleset reloaded` / `WARN  Failed to load ruleset` /
  `WARN  Failed to reload ruleset` — `cmd/engine/main.go`
- `WARN  Push delivery failed` (endpoint, class) — `internal/service/push/dispatcher.go:229`
- `WARN  Push submission rejected` (reason=retired | buffer_full) — `internal/service/push/dispatcher.go:136,164`

SILENT zones (a stall or drop here produces NO log and NO metric):
1. handler-side eligibility: content==nil, v2 snapshot nil, album
   quiet-period, filter exits — nothing counted.
2. unknown endpoint: the submit is rejected without any log or metric
   (`dispatcher.go:151-155`) — though unreachable with a valid ruleset
   (a rule's `pushTo` must name a configured target or `Load()` fails).
3. worker stuck: only visible via `buffer_depth` growth + flat
   `last_success_unix` (there is no in-flight indicator).
4. **success**: a successful push writes only `last_success_unix` and no log
   line (`dispatcher.go:230-234`) — with no metrics backend, "pushed
   successfully" and "never submitted" are indistinguishable from the logs.

## Ruleset reload / generation semantics

- Hot reload: fsnotify → `rulesetRepo.Load()` → on error: WARN
  `reload_failed`, the OLD snapshot keeps serving; on success:
  `loadSnapshot` → `pushManager.Replace(configs, publish)`.
- `Replace` (`internal/service/push/manager.go:24`) order: build candidate
  generation → `publish(candidate)` swaps the handler's submitter
  ATOMICALLY → swap `m.current` → `previous.Retire()`. Retire rejects new
  leases, waits for active submitters, then closes and drains each queue
  (in-flight items are still delivered).
  ⇒ A reload cannot silently drop queued items. A failed reload keeps the
  old snapshot serving (stale config, not a stop). A successful reload
  leaves a live worker.

## Attempt semantics

- `timeoutSeconds` (60 in this deployment) bounds ONE attempt; there is no
  retry on the POST.
- A hanging endpoint therefore costs one dropped event per message
  (at-most-once), classified `timeout`; a slow consumer degrades into silent
  loss, and the only trace is `delivery_failed_total{class=timeout}`.
- Delivery success is a 2xx response; `last_success_unix` advances only on
  success.
