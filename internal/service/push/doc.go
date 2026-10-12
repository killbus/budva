// Package push defines external delivery of forwarder events.
//
// Delivery is at-most-once and events may be lost. Events are self-contained:
// an edited event carries the full current text. Order is not guaranteed;
// consumers should apply last-write-wins using occurred_at and tolerate events
// without predecessors. Deleted events carry identifiers and an empty text, so
// consumers must cache received messages themselves to render deletions.
//
// A submission can be rejected because its endpoint buffer is full or its
// dispatcher generation is retiring. An accepted event receives one HTTP
// attempt, which may fail and is never retried. A process boundary can truncate
// the in-memory stream: restart truncates the stream; no durability.
// No persistence is built. Retirement drains accepted
// events without racing active senders. Each endpoint has one worker and a
// default buffer of 256; at the default 5s timeout retirement's conservative
// bound is 257 attempts times 5s, with endpoints draining concurrently.
package push
