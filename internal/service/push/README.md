# External Push v1

Configure named `pushTargets` in the ruleset and add `pushTo: [name]` to a
forward rule. Targets accept canonical JSON, not native ntfy/Bark/Slack payloads;
product-specific translators are not part of v1.

```yaml
pushTargets:
  archive:
    url: https://receiver.example/events
    timeoutSeconds: 5
    headers:
      Authorization: Bearer REPLACE_LOCALLY
    # hmacSecret: REPLACE_LOCALLY  # optional, default off
forwardRules:
  example:
    from: 1001000
    to: [2001000]
    sendCopy: true
    pushTo: [archive]
```

Treat ruleset URLs, headers and signing keys as sensitive. Keep real settings
out of git; mount a private deployment ruleset with `chmod 0600 ruleset.yml`.
Only HTTPS endpoints are accepted, TLS verification is mandatory, and redirect
following is capped at three hops with HTTPS downgrade forbidden. Optional HMAC
uses `X-Budva-Signature: sha256=<hex>` over the exact JSON request body.

Each endpoint has a single independent worker, a 256-event memory buffer and a
5-second default attempt timeout. Submission never waits for network I/O.
Full buffers and retired snapshots reject submissions; accepted deliveries
receive one attempt without retry or circuit breaker. Only final 2xx responses
count as success. Reload publishes a complete snapshot before draining its
predecessor; queued deliveries retain their original endpoint settings.
Graceful drain has a conservative bound of `(256 + 1) * timeout` per endpoint,
with endpoints draining concurrently. An abrupt process boundary loses queued
events: restart truncates the stream; no durability.

The envelope contains `schema_version: 1`, `event: new|edited|deleted`, `chat_id`,
`message_id`, UTC `occurred_at`, and `text`. Edited events contain full current
source text; deleted events contain empty text. Consumers must tolerate missing
predecessors, treat events as unordered, apply last-write-wins by `occurred_at`,
and cache their own history to render deletions. No raw TDLib payload is sent.

New single-message events use the existing `FiltersOK` path before Telegram
destinations. Album members emit supported text/caption after the existing
collection quiet period, evaluating `FiltersOK` per member for push only.
Empty captions are valid; duplicate `(chat_id, message_id)` members within a
rule's album bucket emit once. Telegram album batching, forwarding, mapping and
statistics do not change, even when a member fails the push filter. Album new
events share the bucket's first-ingress timestamp, not collection completion time, so
delayed old captions cannot win over subsequent edits/deletes in downstream LWW.
No per-member receipt-time state is added. New events retain the existing
supported-content and saveability guards; protected-message
routing and rich media transport are not extended by this correction.

Edited and permanent deleted events use the captured source-rule push routes,
even without Telegram copy mappings or destinations. Edits carry full supported
text/caption (including empty captions) and do not re-run new-message filters:
clearing a caption or leaving a filter must still update a consumer's old copy.
Edits without a matching new event are possible; no eligibility history is
stored. Push events are not repeated by Telegram retries, and queued album/edit
work retains its captured dispatcher across reload. This remains a text/caption
test protocol, not full Telegram content, entities, media or relationship support.

`budva.push.*` metrics use the existing global OTel meter and are exported with
the engine's `METRICS_PORT`. Counters distinguish enqueue, buffer rejection,
retired rejection, classified attempt failure and reload outcome; gauges expose
buffer depth and actual last-success time (zero before the first 2xx); send
duration is a histogram. Process-boundary loss cannot be reliably counted.
Logs contain endpoint names and outcome classes, never URL/header/body secrets.

Persistence or an MQ sink is deferred until replay, a second full-stream
consumer, or an already-operated downstream durable bus requires it.
