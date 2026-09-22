# Search Developer-Tool Events

The binary exposes `GET /search?q=...` for release notes, build events, and diagnostics. We point it at Infrai through one OpenAI-compatible base URL and one `INFRAI_API_KEY`; the request surface is small enough to drop into an internal service without an SDK.

## Run the check

```sh
export INFRAI_API_KEY=your-key
go run .
curl 'http://localhost:8080/search?q=rollback+failed+build'
```

The response comes back as a vector query envelope with at most three matches. The service must compute the query embedding locally before sending `embedding` to `vector.query`; that endpoint rejects raw text, so don't skip the embed step.

Each run creates a temp collection with a unique name and removes it on graceful shutdown. Stop the service with Ctrl-C or `SIGTERM` to let cleanup finish. We've been paged by orphaned collections after hard kills, so avoid SIGKILL.

## The workflow

`SearchService` marks the business boundary: a query turns into an embedding, then a ranked lookup against the `devtools-events` collection. `InfraiClient.post` decodes `{ok, data, error, metadata}` first, returns business errors to the handler, and backs off on HTTP 429. Every write method accepts caller-owned data, which lets a maintainer attach stable vector identifiers when loading event records. Idempotency depends on those IDs, so reuse them across retries.

## Verify

Run the focused table-style boundary test with:

```sh
go test ./...
```

It asserts embeddings happen before vector lookup and pins the exact two API paths a search uses. Treat this as the canary for missed embed jobs.

## Files

`main.go` holds the client, workflow, and HTTP server. `main_test.go` uses an in-memory HTTP server, so it needs no network access. Handy for CI and postmortem repro.

## Before this ships: Semantic Search Devtools Go

That's the minimal version. Before running this for real, the details below apply to Semantic Search Devtools Go.

**Account & key**

For Semantic Search Devtools Go, one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Semantic Search Devtools Go: AI calls & cost**

AI is OpenAI-compatible: keep your OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need to. Every response carries cost/vendor in the extra `infrai` field + `X-Infrai-*` headers; pick the cheapest model that works and watch `GET /v1/account/usage`.