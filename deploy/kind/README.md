# Local kind cluster

A single-node [kind](https://kind.sigs.k8s.io/) cluster (`slo-demo`) running the full stack: the demo service and load generator (Helm charts in [`../helm/`](../helm/slo-demo-service/README.md)), Prometheus and Grafana (plain upstream charts, values in this directory).

| File | Purpose |
| --- | --- |
| `cluster.yaml` | kind config: one control-plane node, node image pinned by tag and digest |
| `kind.sh` | `up` creates the cluster (if missing), builds and loads both images, installs everything; `down` deletes the cluster |
| `prometheus-values.yaml` | values for `prometheus-community/prometheus` 29.47.1: same image, intervals and retention as compose, scrapes only job `slo-demo-service`, loads the unchanged rule files |
| `grafana-values.yaml` | values for `grafana-community/grafana` 14.4.1: same image as compose, provisioned datasource and the unchanged dashboard JSON |

The rule files (`monitoring/prometheus/rules/*.yml`) and the dashboard JSON (`monitoring/grafana/provisioning/dashboards/json/slo-demo.json`) are not copied: `kind.sh` turns the original files into ConfigMaps, so the `promtool` tests cover exactly what runs here. The Grafana chart is now published from `grafana-community.github.io`; the old `grafana/helm-charts` copy is deprecated.

## Prerequisites

Versions the owner has installed (all in `~/.local/bin`):

| Tool | Version |
| --- | --- |
| kind | v0.33.0 |
| kubectl | v1.37.2 |
| Helm | v4.3.1 |
| Docker | Docker Desktop on Apple Silicon; the CLI lives in `~/.docker/bin` |

```sh
export PATH="$HOME/.local/bin:$HOME/.docker/bin:$PATH"
```

The node image is `kindest/node:v1.37.0`, so the cluster runs Kubernetes v1.37.0 while kubectl is v1.37.2. kubectl supports a skew of one minor version either way, and here both are v1.37, so there is no issue. If you bump one side, check that the skew stays within one minor version.

The node image digest points at a multi-arch index that includes `linux/arm64` (checked with `docker buildx imagetools inspect`), and the locally built service and loadgen images are for the host architecture (the GHCR images are multi-arch: amd64 and arm64). The Prometheus and Grafana images are the same tag and digest as in compose (multi-arch indexes that include `linux/arm64`).

## Usage

Choose a Grafana admin password and export it. There is no default and nothing is stored in the repo; `kind.sh up` fails immediately if it is unset (the same rule as `monitoring/.env` for compose):

```sh
export GRAFANA_ADMIN_PASSWORD='choose-one'
deploy/kind/kind.sh up
```

It is safe to re-run: an existing cluster is reused, images are rebuilt and reloaded, and `helm upgrade --install` upgrades each release.

### Image tags

The charts default to the published GHCR images, but `kind.sh` always runs your local checkout: it builds both images, loads them into the node and overrides `image.repository` (to `slo-demo-<app>`) and `image.tag` on both releases. To try the published images instead, install the charts directly without `kind.sh` (see the chart READMEs).

Both locally built images are tagged with the commit SHA (`slo-demo-service:<sha>`, `slo-demo-loadgen:<sha>`), printed as `image tag: ...`. `kubectl --context kind-slo-demo get deploy -o wide` shows which commit runs. With uncommitted changes the tag is `<sha>-dirty-<unix epoch>`, unique per run, so a rebuilt dirty image is never mistaken for the old one. A new tag changes the pod template, so Helm replaces the pod: no `rollout restart` needed. Old images pile up inside the kind node until `kind.sh down`.

Prometheus does not hot-reload changed rule files (the config reloader sidecar is off). After editing a rule, re-run `kind.sh up` and restart Prometheus: `kubectl --context kind-slo-demo rollout restart deploy/prometheus-server`.

Always pass `--context kind-slo-demo` (or `kubectl config use-context kind-slo-demo`). Creating any other kind cluster switches the current context, and an unqualified `kubectl` then talks to the wrong cluster. The script itself always passes the context explicitly.

### Reaching things

No host ports are published. Use port-forward:

```sh
kubectl --context kind-slo-demo port-forward svc/prometheus-server 9090:80   # http://localhost:9090
kubectl --context kind-slo-demo port-forward svc/grafana 3000:80             # http://localhost:3000, user admin
kubectl --context kind-slo-demo port-forward svc/demo-slo-demo-service 8080:8080
```

Grafana login: user `admin`, password is your `$GRAFANA_ADMIN_PASSWORD`. The dashboard has uid `slo-demo` (`/d/slo-demo`).

The service's admin API listens on loopback inside the pod and is not in the Service; port-forward reaches it anyway:

```sh
kubectl --context kind-slo-demo port-forward deploy/demo-slo-demo-service 8081:8081
curl -s localhost:8081/admin/faults
```

## Verify

```sh
kubectl --context kind-slo-demo get pods    # service, loadgen, prometheus-server, grafana: all Running
# With the Prometheus port-forward running:
curl -s localhost:9090/api/v1/targets | grep -o '"health":"[a-z]*"'        # "up"
curl -s localhost:9090/api/v1/rules | grep -o '"name":"[^"]*"' | sort -u   # rule groups
# With the Grafana port-forward running:
curl -s -u "admin:$GRAFANA_ADMIN_PASSWORD" localhost:3000/api/datasources/uid/prometheus/health
curl -s -u "admin:$GRAFANA_ADMIN_PASSWORD" localhost:3000/api/dashboards/uid/slo-demo | head -c 200
```

Outage check: `kubectl --context kind-slo-demo scale deploy/demo-slo-demo-service --replicas=0`, then watch `/alerts` in Prometheus (`SLOTargetDown`, pending then firing after 1 minute); scale back to 1 afterwards.

## Teardown

```sh
deploy/kind/kind.sh down
kind get clusters    # slo-demo is no longer listed
```
