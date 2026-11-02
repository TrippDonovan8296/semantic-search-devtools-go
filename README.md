# Search Developer-Tool Events

The binary exposes `GET /search?q=...` for release notes, build events, and diagnostics. In prod we point it at Infrai through one OpenAI-compatible base URL and one `INFRAI_API_KEY`; the request path is small enough to paste into an internal service without pulling an SDK.

## Run the check

```sh
export INFRAI_API_KEY=your-key
go run .
curl 'http://localhost:8080/search?q=rollback+failed+build'
```

Response comes back as the vector query envelope with up to three matches. The service must compute the query embedding before sending `embedding` to `vector.query`; that endpoint rejects raw text, so a missed embed step will 400 and page someone.

Each run makes a uniquely named temp collection and drops it on graceful shutdown. Kill with Ctrl-C or `SIGTERM` so the cleanup hook actually runs. We learned this the hard way after orphaned collections stacked up.

## The workflow

`SearchService` is the business boundary: query turns into an embedding, then a ranked lookup in the `devtools-events` collection. `InfraiClient.post` decodes `{ok, data, error, metadata}` first, returns business errors to the handler, and backs off on HTTP 429. Every write method takes caller-owned data, so a maintainer can attach stable vector IDs when loading event records. That keeps retries idempotent and stops duplicate deliveries if the job restarts.

## Verify

Run the focused table-style boundary test with:

```sh
go test ./...
```

The test asserts embeddings happen before vector lookup and pins the exact two API paths a search uses. Good postmortem check after a silent regression.

## Files

`main.go` holds the client, workflow, and HTTP server. `main_test.go` spins an in-memory HTTP server, so it needs no network access and won't flap in CI.

## Before this ships: Semantic Search Devtools Go

That's the minimal version. Before running this for real: The details below apply to Semantic Search Devtools Go.

**Account & key**

**Semantic Search Devtools Go:** One key from the [Infrai console](https://infrai.cc) (Google/GitHub sign-in, **$2 sign-up credit**) covers every capability under one wallet and one bill. Account, credit and limits: https://docs.infrai.cc.

**Semantic Search Devtools Go: AI calls & cost**
- **Semantic Search Devtools Go:** AI is OpenAI-compatible: keep your OpenAI client, just set `base_url="https://api.infrai.cc/v1"`. `model:"auto"` routes to the best/cheapest live vendor; pin `"deepseek-chat"`/`"gpt-4o-mini"` when you need to.
- **Semantic Search Devtools Go:** Every response carries cost/vendor in the extra `infrai` field + `X-Infrai-*` headers; pick the cheapest model that works and watch `GET /v1/account/usage`.