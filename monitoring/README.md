# Monitoring stack

Local Docker Compose stack: the demo service (built from `service/Dockerfile`),
Prometheus scraping its `/metrics`, and Grafana with a provisioned Prometheus
datasource. No recording rules, alerts or dashboards yet. Those depend on the
owner's SLO decisions (D1-D6 in `docs/PLAN.md`). The load generator is not part
of the stack yet.

## Setup

Grafana's admin password is never committed. Create the git-ignored
`monitoring/.env` from the example and set a real password:

```sh
cp monitoring/.env.example monitoring/.env
# edit monitoring/.env: GRAFANA_ADMIN_PASSWORD=<something real>
```

Without it, `docker compose` refuses to start rather than falling back to
Grafana's default `admin`/`admin`.

## Run

```sh
docker compose -f monitoring/docker-compose.yml up -d --build
docker compose -f monitoring/docker-compose.yml ps
```

All ports are published on `127.0.0.1` only:

| Port | What |
|---|---|
| 8080 | service API and `/metrics` |
| 8081 | service admin API (fault injection, no auth) |
| 9090 | Prometheus |
| 3000 | Grafana (login from `monitoring/.env`) |

Check that the scrape works:

```sh
curl -s localhost:8080/api/fast >/dev/null
curl -s localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health}'
curl -s localhost:9090/api/v1/query --data-urlencode 'query=http_requests_total' | jq '.data.result'
```

`http_requests_total` has no series until the service has served at least one
request. The scrape interval (15s) is a placeholder; see `prometheus/prometheus.yml`.

## Startup

`up -d` returning does not mean the stack is ready. The service has no
healthcheck (distroless image, no shell), so for the first seconds Prometheus
may not answer yet, and then reports `slo-demo-service` as `unknown` (not
scraped yet) or `down` until a scrape succeeds. Wait for:

```sh
curl -s localhost:9090/api/v1/targets | jq -c '.data.activeTargets[] | {job: .labels.job, health}'
# expect: {"job":"slo-demo-service","health":"up"}
```

`restart: unless-stopped` restarts the service after a crash, and its counters
then start from zero. `rate()` handles this as a counter reset, but keep it in
mind for the burn-rate rules in Phase 2.

## Stop

```sh
docker compose -f monitoring/docker-compose.yml down      # keep Prometheus/Grafana data
docker compose -f monitoring/docker-compose.yml down -v   # also delete the data volumes
```
