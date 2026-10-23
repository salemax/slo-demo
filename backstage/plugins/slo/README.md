# slo

Frontend-only Backstage plugin showing the SLO state of a catalog entity.
It is the Phase 3 plugin from `docs/PLAN.md`, built the way D8 decided:
no backend plugin, Prometheus is read through the Backstage proxy.

## What it shows

One SLO card on the entity page, with a row per `slo` label the entity
declares, and four columns. It is registered as an entity card of
`type: 'content'`, not `'info'`: an `'info'`-type card sits in the narrow
right-hand column (measured 330 px), where the five columns (the row label
plus the four below) wrap onto three lines each; in the content area the
same table is 661 px wide and fits on one line. The card itself still
renders with Backstage's `InfoCard` component from `@backstage/core-components`
— that name is unrelated to the `'info'`/`'content'` placement type.

| column         | series it reads                            |
| -------------- | ------------------------------------------ |
| Target         | `slo:objective:ratio`                      |
| Compliance     | `1 - slo:sli_error:ratio_rate28d`          |
| Budget left    | `slo:error_budget_remaining:ratio_rate28d` |
| Burn rate (5m) | `slo:error_budget_burn_rate:ratio_rate5m`  |

Those are recording rules from `monitoring/prometheus/rules/slo-recording.yml`.
The card does no SLI arithmetic of its own beyond `1 - error ratio`, so the
SLO definitions (D1-D6) stay in one place.

## How an entity opts in

```yaml
metadata:
  annotations:
    slo-demo/slos: availability,latency # presence shows the card, values are the rows
    slo-demo/grafana-dashboard-uid: slo-demo # optional, adds the dashboard link
```

Entities without `slo-demo/slos` get no card at all: the extension's
filter is the `hasSlos` predicate in `src/annotations.ts`.

## No data

A missing series and a `NaN` value both render as "No data", never as 0 and
never as green. An idle service records no requests, so its ratios are 0/0,
which Prometheus returns as `NaN`; that is not 100% availability. An
unreachable Prometheus replaces the table with the error.

The card fetches once when the page mounts, and does not poll. Watch a burn
live on the Grafana dashboard instead.

## Configuration

- `proxy.endpoints./prometheus/api` in `app-config.yaml` — where Prometheus is.
- `slo.grafanaBaseUrl` (optional) — base url for the dashboard link. Absent
  means the card renders without a link.

## Placeholders

Decimal places, the column set, the "No data" wording and the absence of a
refresh interval are this plugin's choices, not decisions from
`docs/PLAN.md`.

## Tests

```bash
yarn workspace @internal/backstage-plugin-slo test
```

The card's only I/O goes through `fetchApi`, so the tests stub that rather
than running a mock HTTP server.
