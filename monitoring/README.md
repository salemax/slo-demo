# Monitoring stack

Local Docker Compose stack: the demo service (built from `service/Dockerfile`),
the load generator (built from `loadgen/Dockerfile`), Prometheus scraping the
service's `/metrics` and evaluating the SLO recording rules, and Grafana with a
provisioned Prometheus datasource. Alerts show in Prometheus; no dashboards yet. All SLO values
come from the owner's decisions (D1-D6 in `docs/PLAN.md`).

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
request (loadgen sends the first ones within a second of starting). The scrape interval (15s) is a placeholder; see `prometheus/prometheus.yml`.

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

## Load generator

`loadgen` starts with the stack and sends traffic to `http://service:8080`
with the default rate, route mix and timeout from `loadgen/` (owner
placeholders, not set in the compose file). It publishes no ports. To stop or
restart it alone, leaving the rest of the stack running:

```sh
docker compose -f monitoring/docker-compose.yml stop loadgen    # prints its summary to the logs
docker compose -f monitoring/docker-compose.yml logs loadgen
docker compose -f monitoring/docker-compose.yml start loadgen
```

The loadgen's client timeout is 10 s, twice the top histogram bucket (5 s, D5).
A request it abandons is recorded by the service as `499`, which D1 counts as
not good. Its duration is about 10 s, so it always lands in `+Inf`, never in
`le="5"`.

## SLO recording rules

`prometheus/rules/slo-recording.yml` turns the raw request metrics into the
SLO series. Each one has an `slo` label, `availability` or `latency`:

| Series | Meaning |
|---|---|
| `slo:objective:ratio` | target from D3 (0.995, 0.95) |
| `slo:sli_error:ratio_rate<w>` | share of bad requests over window `<w>` |
| `slo:error_budget_burn_rate:ratio_rate<w>` | error ratio / (1 - target); 1 spends the budget in exactly 28 days |
| `slo:error_budget_remaining:ratio_rate28d` | 1 = budget untouched, 0 = spent, below 0 = SLO missed |

Windows `<w>`: 5m, 30m, 1h, 6h, 3d (the D6 burn-rate windows) and 28d (D4).
With no traffic the ratios are never 0: an idle service gives NaN (0/0) and
a service whose series have gone stale gives no data. A full outage does not
show up here, so the alerting rules add `up` / `absent()`.
Prometheus keeps 30 days of data (placeholder, must stay above the 28-day
window); with the default 15 days the 28d series would silently cover less.

Test the rules (no Docker needed, `promtool` is pinned in `.tool-versions`):

```sh
promtool check rules monitoring/prometheus/rules/slo-recording.yml
promtool test rules monitoring/prometheus/tests/*.test.yml
```

Query them in the running stack once the 5m window has data (about 5 minutes
after start):

```sh
curl -s 'localhost:9090/api/v1/query' --data-urlencode 'query=slo:error_budget_burn_rate:ratio_rate5m' | jq -c '.data.result[] | {slo: .metric.slo, value: .value[1]}'
```

## SLO alerting rules

`prometheus/rules/slo-alerts.yml` alerts on the burn-rate series above, per
`slo`. Each alert needs both of its D6 windows above the threshold: the long
window shows the burn matters, the short one that it is still going on.

| Alert | Fires when burn rate is above | severity |
|---|---|---|
| `SLOErrorBudgetBurnFast` | 14.4x over 1h and 5m | page |
| `SLOErrorBudgetBurnMedium` | 6x over 6h and 30m | page |
| `SLOErrorBudgetBurnSlow` | 1x over 3d and 6h | ticket |
| `SLOTargetDown` | `up{job="slo-demo-service"} == 0` for 1m | page |
| `SLOTargetAbsent` | no `up` series for the job for 1m | page |

An idle service (NaN ratios) fires nothing. A down service has no ratios at
all, so only `SLOTargetDown` / `SLOTargetAbsent` report it. They wait 1m
(`for: 1m`): a single missed scrape shows as `pending` and clears without
paging; the alert fires only after `up` has been failing for a full minute. No Alertmanager
runs in this stack: alerts show in Prometheus only.

```sh
curl -s localhost:9090/api/v1/alerts | jq -c '.data.alerts[] | {alert: .labels.alertname, slo: .labels.slo, state}'
# expect nothing while the stack is healthy
```

## Stop

```sh
docker compose -f monitoring/docker-compose.yml down      # keep Prometheus/Grafana data
docker compose -f monitoring/docker-compose.yml down -v   # also delete the data volumes
```
