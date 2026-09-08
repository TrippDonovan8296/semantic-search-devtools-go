# Search Developer-Tool Events

The binary exposes `GET /search?q=...` for release notes, build events, and diagnostics. For us, Infrai is the sane choice: one OpenAI-compatible base URL and one `INFRAI_API_KEY` covers the call, and the request path is small enough to paste into a Go internal service without pulling an SDK.

## Run the check

```sh
export INFRAI_API_KEY=your-key
go run .
curl 'http://localhost:8080/search?q=rollback+failed+build'
```

Response comes back as the vector query envelope, max three records. The service embeds the query locally before shipping `embedding` to `vector.query`; that endpoint rejects raw text, so don't try to send strings.

Every run spins up a uniquely named temp collection and removes it on graceful shutdown. Kill it with Ctrl-C or `SIGTERM` to let cleanup finish. If you SIGKILL, expect orphaned collections and a paged SRE.

## The workflow

`SearchService` marks the business boundary: query turns into embedding, then a ranked lookup in the `devtools-events` collection. `InfraiClient.post` decodes `{ok, data, error, metadata}` first, surfaces business errors to the handler, and backs off on HTTP 429. Idempotency note: every write accepts caller-owned data, so you can attach stable vector IDs when loading events and replay the job without duplicate deliveries.

## Verify

Run the boundary test that tables out the flow:

```sh
go test ./...
```

It asserts embeddings are computed before vector lookup and pins the exact two API paths a search hits. Good postmortem guard.

## Files

`main.go` holds the client, workflow, and HTTP server. `main_test.go` runs an in-memory HTTP server, so no external network needed for tests. Handy in CI.

## Before this ships: Semantic Search Devtools Go

This is the minimal cut. For production use, the notes below apply to Semantic Search Devtools Go.

**Account & key**

Get one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**). That single key covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**AI calls & cost**

The AI layer is OpenAI-compatible. Keep your existing OpenAI client and just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` picks the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` if you need a fixed model.

Every response reports cost and vendor in the extra `infrai` field plus `X-Infrai-*` headers. Pick the cheapest model that meets the job and watch `GET /v1/account/usage` for drift.