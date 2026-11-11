# Local kind cluster

A single-node [kind](https://kind.sigs.k8s.io/) cluster (`slo-demo`) running the demo service from the Helm chart in [`../helm/slo-demo-service`](../helm/slo-demo-service/README.md). Monitoring (Prometheus, Grafana) on the cluster is not part of this directory yet.

| File | Purpose |
| --- | --- |
| `cluster.yaml` | kind config: one control-plane node, node image pinned by tag and digest |
| `kind.sh` | `up` creates the cluster (if missing), builds and loads the image, installs the chart; `down` deletes the cluster |

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

The node image digest points at a multi-arch index that includes `linux/arm64` (checked with `docker buildx imagetools inspect`), and the service image is built for the host architecture.

## Usage

From the repository root:

```sh
deploy/kind/kind.sh up
```

It is safe to re-run: an existing cluster is reused, the image is rebuilt and reloaded, and `helm upgrade --install` upgrades the release. Because the image tag is always `slo-demo-service:dev` and the pull policy is `IfNotPresent`, running pods keep the old image after a rebuild; restart them:

```sh
kubectl --context kind-slo-demo rollout restart deploy/demo-slo-demo-service
```

Always pass `--context kind-slo-demo` (or `kubectl config use-context kind-slo-demo`). Creating any other kind cluster switches the current context, and an unqualified `kubectl` then talks to the wrong cluster. The script itself always passes the context explicitly.

No host ports are published. Reach the service with port-forward:

```sh
kubectl --context kind-slo-demo port-forward svc/demo-slo-demo-service 8080:8080
curl -s localhost:8080/api/fast
curl -s localhost:8080/metrics | grep http_requests_total
```

The admin API listens on loopback inside the pod and is not in the Service. Port-forward reaches it anyway, because the kubelet connects to the pod's network namespace:

```sh
kubectl --context kind-slo-demo port-forward deploy/demo-slo-demo-service 8081:8081
curl -s localhost:8081/admin/faults
```

## Verify

```sh
kubectl --context kind-slo-demo get nodes -o wide     # one node, Ready, v1.37.0, (arm64)
kubectl --context kind-slo-demo get pods              # demo-slo-demo-service-... 1/1 Running
```

## Teardown

```sh
deploy/kind/kind.sh down
kind get clusters    # slo-demo is no longer listed
```
