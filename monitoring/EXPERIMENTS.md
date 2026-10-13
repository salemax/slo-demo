# SLO experiments

Four experiments that inject faults into the running stack and record what the
SLO series, the dashboard and the alerts do. Every number below was measured
on 2026-10-13 on a fresh stack (amd64 agent VM, loadgen at its default
20 req/s); the "expected" values are worked out by hand from D1-D6 before
looking at Prometheus.

Run them in this order: each one leaves history in the windows that the next
one reads, and the expected values below assume that history.

## Read this first: a young stack is not production

Every window (`5m` ... `28d`) can only contain the data Prometheus has. On a
stack that is 20 minutes old, the "6h", "3d" and "28d" windows all contain
the same 20 minutes. Two consequences you will see in every experiment:

- **Long windows react almost as fast as short ones.** The slow ticket
  (1x over 3d and 6h) fired 1.4 minutes into a 5% error experiment. In
  production, 3 days of clean history would dilute those minutes and nothing
  would fire.
- **The 28d budget collapses far below 0%.** Ten minutes of 5% errors on a
  20-minute-old stack is "5% errors for half the window", 5x the 0.5%
  allowance. In production the same ten minutes are 10 of 40,320 minutes:
  1/4032 of the window at 10x the allowed rate, so 10/4032, about 0.25% of the budget.

The multi-window alerts and the budget math are correct; the inputs are just
short. Read the burn-rate numbers as "how fast", and treat the long-window
alerts and the 28d budget as demonstrations of the formula, not of real
consequences.

## Setup

```sh
cp monitoring/.env.example monitoring/.env    # once; set GRAFANA_ADMIN_PASSWORD
docker compose -f monitoring/docker-compose.yml down -v   # fresh Prometheus data
docker compose -f monitoring/docker-compose.yml up -d --build
```

Wait about 10 minutes before the first experiment. This clean history is what
keeps experiment 1 below the medium page threshold (see its expected values).

Dashboard: <http://localhost:3000/d/slo-demo>, time range "Last 1 hour".

Helpers used below (no auth: everything listens on `127.0.0.1` only):

```sh
faults() { curl -s -X PUT localhost:8081/admin/faults -d "$1"; echo; }
clear_faults() { curl -s -X DELETE localhost:8081/admin/faults -o /dev/null -w '%{http_code}\n'; }
burn() { curl -s localhost:9090/api/v1/query --data-urlencode "query=slo:error_budget_burn_rate:ratio_rate$1" \
  | jq -rc '.data.result[] | [.metric.slo, .value[1]]'; }
alerts() { curl -s localhost:9090/api/v1/alerts | jq -rc '.data.alerts[] | [.labels.alertname, .labels.slo, .state, .activeAt]'; }
```

`burn 5m`, `burn 1h` and so on print one line per SLO.

### The two numbers to keep in mind

- **Burn rate** = error ratio / error budget. The availability budget is
  0.5% (D3: 99.5%), the latency budget 5% (D3: 95%). A burn rate of 1 spends
  the budget in exactly 28 days (D4); 10 spends it in 2.8 days.
- **D6 alerts** need *both* windows above the threshold: 14.4x over 1h and
  5m (page), 6x over 6h and 30m (page), 1x over 3d and 6h (ticket).

## Experiment 1: below the fast-page threshold (5% errors)

The question: does an error rate that is 10x the budget page immediately?
The fast page needs 14.4x, so it must not.

```sh
faults '{"rules":[{"route":"all","error_rate":0.05}]}'
# wait 10 minutes, checking burn 5m / burn 1h / alerts
clear_faults
```

**Expected:** availability burn = 0.05 / 0.005 = **10** on the 5m window.
The long windows hold about 11 clean minutes plus N faulty ones, so their burn
is 10 x N / (11 + N): **4.8** after 10 minutes. 10 < 14.4, so no fast page.
4.8 < 6, so no medium page. The slow ticket needs only > 1 on both of its
windows, so it fires after about 1.2 minutes (young-stack effect).
Latency: untouched, because injected errors return immediately.

**Measured** (fault 08:54:05-09:04:05 UTC):

| after | burn 5m | burn 30m / 1h / 6h | 28d budget left | alerts |
|---|---|---|---|---|
| 5 min | 9.84 | 3.14 | -214% | `SLOErrorBudgetBurnSlow` firing (since 08:55:30) |
| 10 min | 10.67 | 4.95 | -395% | same |

The fast and medium pages never fired. The latency burn rate stayed 0 on every
window.

**What it shows:** a 10x burn is serious (the budget would be gone in 2.8
days), but it is not a page under D6's fast pair. On a stack with long clean history it
would page through the medium pair once the 6h window is above 6x, which takes
about 3.6 hours of 10x (6 / 10 x 6h).

## Experiment 2: fast burn (20% errors)

The question: how long does a 40x burn take to page, and how fast does the
page clear?

```sh
faults '{"rules":[{"route":"all","error_rate":0.2}]}'
# wait 9 minutes, checking burn 5m / burn 1h / alerts every minute
clear_faults
# keep checking for 8 more minutes
```

**Expected:** burn = 0.2 / 0.005 = **40** on the 5m window once it has filled
(about 5 minutes). The fast page also needs the 1h window above 14.4x, which
means a 1h error ratio above 7.2%. The 1h window already holds experiment 1
(about 600 bad requests) and 21 minutes of traffic, so it needs
(0.5 + 0.2 x N) / (21 + N) > 0.072, which gives **N > 7.9 minutes**. The medium
page (6x, so above 3% on 30m and 6h) is already close after experiment 1 and
fires within the first minute. After the fault is removed, the 5m burn falls
below 14.4 once less than 36% of the 5m window is faulty, about **3 minutes**
later.

**Measured** (fault 09:04:13-09:13:21 UTC):

| time | burn 5m | alerts firing |
|---|---|---|
| +1 min | 15.6 | Medium (since 09:05:00), Slow |
| +5 min | 40.7 | Medium, Slow |
| +8 min | 40.5 | **Fast (since 09:12:15)**, Medium, Slow |
| +9 min (fault cleared) | 40.2 | Fast, Medium, Slow; 1h burn 15.6, 28d budget left -1456% |
| clear +3 min | 16.4 | Fast clears at 09:16:15 |
| clear +5 min | 0 | Medium, Slow (1h burn 13.5) |
| clear +26 min | 0 | Medium clears; Slow stays |

**What it shows:** the 5m window was above 14.4x within 1 minute, but the
fast page waited until the 1h window agreed, 8 minutes in, as computed. That
is the point of two windows: the long one proves that enough budget is gone to
justify a page. After the fix, the short window is what clears the page in
3 minutes. Without it, the alert would stay until the 1h window recovered.
The medium page stays until its short (30m) window no longer holds the
faulty minutes: expected about 25 minutes after the fix, measured 26 (cleared
between 09:39:00 and 09:39:27). The slow ticket stays until
the 6h window rolls past the experiment.

## Experiment 3: latency only (+100 ms on `/api/slow`)

The question: do the two SLOs move independently?

```sh
faults '{"rules":[{"route":"/api/slow","latency_ms":100}]}'
# wait 10 minutes
clear_faults
```

**Expected:** `/api/slow` normally takes 50-250 ms (uniform), always under the
300 ms threshold (D2). With +100 ms it takes 150-350 ms, so a request is
slower than 300 ms when its base delay is over 200 ms: about **25%** of
`/api/slow`. `/api/slow` is about 20% of loadgen traffic, so about 5% of all
requests are slow, which is a latency burn of 0.05 / 0.05 = about **1**.
Availability: no change, because nothing fails.

**Measured** (fault 09:21:31-09:31:28 UTC), at +10 minutes:

| | burn 5m | burn 30m | burn 1h / 6h | 28d budget left |
|---|---|---|---|---|
| latency | 1.07 | 0.35 | 0.22 | 78% |
| availability | 0 | (still experiment 2) | | |

Share of requests over 300 ms over the last 5m, per route: `/api/fast` 0,
`/api/slow` 0.26.

```sh
curl -s localhost:9090/api/v1/query --data-urlencode \
  'query=1 - sum by (route) (rate(http_request_duration_seconds_bucket{le="0.3"}[5m])) / sum by (route) (rate(http_request_duration_seconds_count[5m]))' \
  | jq -rc '.data.result[] | [.metric.route, .value[1]]'
```

No latency alert fired: the slow ticket needs the 6h window above 1, and it
held only 0.22.

**What it shows:** latency and availability are separate budgets. A
degradation that every user feels (one in four slow requests is slower) costs
about 1x on the latency SLO and nothing on availability. At about 1x it
uses up the 28-day budget in 28 days, so it never pages. In production it
would open a ticket only once both the 6h and the 3d windows are above 1x,
that is after about 3 days of it.

## Experiment 4: outage (service stopped for 1 minute)

The question: what does a full outage look like, given that a stopped service
produces no request metrics at all?

```sh
docker compose -f monitoring/docker-compose.yml stop service
# wait 60 seconds, checking alerts and up{job="slo-demo-service"} every 5 s
docker compose -f monitoring/docker-compose.yml start service
```

**Expected:** the next scrape after the stop fails, so `up` drops to 0 within
15 s (scrape interval). `SLOTargetDown` goes `pending` at the next rule
evaluation and fires only after `up == 0` has held for 1m (`for: 1m`). The
service is down for about 60 s, but the first failed scrape comes up to 15 s
after the stop and the first good one up to 15 s after the start. So
`up == 0` is visible for somewhere between about 45 and 75 s, and **whether
the alert fires depends on where the stop falls between scrapes.**

**Measured** (stop 09:31:39, start 09:32:40 UTC):

| time | `up` | `SLOTargetDown` |
|---|---|---|
| 09:31:44 | 0 | - |
| 09:31:49 | 0 | pending |
| 09:32:40 | 0 | pending (start issued) |
| 09:32:45 | 1 | pending |
| 09:32:50 | 1 | cleared, **never fired** |

Loadgen saw about 1,220 failed requests (connection errors, `errors=` in
`docker compose logs loadgen`). **The availability burn rate stayed at 0 for
the whole outage.**

**What it shows:**

- `for: 1m` did what it is for: this one-minute outage stayed `pending` and
  did not page. A run whose stop falls just after a scrape can fire. Stop for
  2 minutes to see it fire every time, or 30 s to see it reliably stay
  `pending`.
- **An outage costs no error budget in this setup.** The SLI is measured
  inside the service (`http_requests_total`). A request that cannot reach the
  service is never counted, as either good or bad. The ratios do not drop;
  they keep their last values or go stale. That is why
  `SLOTargetDown`/`SLOTargetAbsent` exist, and it is also a limit of the D1
  SLI: a 1-hour outage pages, but the 28d SLI still reads 100%. See the PR
  for the open question.

## Clean up

```sh
clear_faults
docker compose -f monitoring/docker-compose.yml down -v   # -v drops the experiment history
```
