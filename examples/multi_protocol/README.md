# multi_protocol — Tester DSL across HTTP / GraphQL / SSE / SOAP

Companion to `examples/kitchen_sink/` that proves the Go SDK's Tester
DSL chain works across four protocols in one job, with **zero
external infrastructure** beyond the testbackend.

| Step | Facet | What it proves |
|------|-------|----------------|
| 1    | HTTP | GET against `/api/v1/token-chain/issue` → seed `{{token}}` |
| 2    | GraphQL | Typed query with variables (`$id: ID!`), `ExpectNoErrors`, JSONPath assert |
| 3    | SSE | Subscribe to `/events/notifications`, listen 2 s, assert ≥ 1 event arrived |
| 4    | SOAP | POST a SOAP envelope (`GetUser` op), `ExpectStatus` + `ExpectNoFault` |

Verified live: `multi_protocol: ok — http+graphql+sse+soap` end-to-end
against a running testbackend, ~2.2 s total (SSE listen window
dominates).

## Run it

```bash
mockarty-testbackend &
TESTBACKEND_URL=http://127.0.0.1:18770 go run .
```

## Why this sibling exists

`kitchen_sink/` is the canonical adopter showcase: HTTP + GraphQL +
upstream-tracker side-channels + TCM upload. `multi_protocol/`
narrows the focus to **just the Tester DSL chain across protocols**
— useful when the question is "does the Tester really work over SSE
/ SOAP without docker-spun brokers?" The answer ships as one runnable
example.

For Kafka / RabbitMQ / gRPC / DB, the dedicated examples have
docker-compose snippets:

  - `examples/kafka_client/`
  - `examples/rabbitmq_client/`
  - `examples/grpc_client/`

Those facets need real brokers — keeping them out of this example
keeps the showcase 100% offline-runnable.
