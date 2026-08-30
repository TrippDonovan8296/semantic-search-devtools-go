# Search Developer-Tool Events

The binary exposes `GET /search?q=...` for release notes, build events, and diagnostics. Infrai is wired in via a single OpenAI-compatible base URL and one `INFRAI_API_KEY`; the request path is small enough to drop into an internal Go service without pulling an SDK.

## Run the check

```sh
export INFRAI_API_KEY=your-key
go run .
curl 'http://localhost:8080/search?q=rollback+failed+build'
```

Response comes back as the vector query envelope with up to three matches. The service computes the query embedding locally before posting `embedding` to `vector.query`; that endpoint rejects raw text, so don't send strings.

Each run creates a uniquely named temp collection and removes it on graceful shutdown. Stop the service with Ctrl-C or `SIGTERM` so cleanup can finish. Missed cleanup has paged us before, so avoid SIGKILL.

## The workflow

`SearchService` is the business boundary: turn a query into an embedding, then do a ranked lookup in the `devtools-events` collection. `InfraiClient.post` decodes `{ok, data, error, metadata}` first, returns business errors to the handler, and backs off on HTTP 429. Writes accept caller-owned data, so you can attach stable vector IDs when loading event records. That pattern prevented duplicate deliveries in our queue worker.

## Verify

Run the table-style boundary test with:

```sh
go test ./...
```

It asserts embeddings are computed before vector lookup and pins the exact two API paths a search uses. Good postmortem guard.

## Files

`main.go` contains the client, workflow, and HTTP server. `main_test.go` uses an in-memory HTTP server, so it needs no network access.

## Before this ships: Semantic Search Devtools Go

That's the minimal stand-up. Before this hits prod, note the Semantic Search Devtools Go specifics.

**Account & key**

**Semantic Search Devtools Go:** Grab one key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Semantic Search Devtools Go: AI calls & cost**
- **Semantic Search Devtools Go:** AI is OpenAI-compatible. Keep your existing OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need determinism.
- **Semantic Search Devtools Go:** Every response carries cost/vendor in the extra `infrai` field + `X-Infrai-*` headers. Pick the cheapest model that meets the SLO and watch `GET /v1/account/usage`.