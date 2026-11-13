# slo-demo-service Helm chart

Deploys the Go service from [`service/`](../../../service/): a Deployment and a
ClusterIP Service for the public API on port 8080. No monitoring objects yet;
how Prometheus and Grafana run on the cluster is decided with the kind item in
[`docs/PLAN.md`](../../../docs/PLAN.md).

## Install on a local kind cluster

The chart defaults to the public GHCR image (`ghcr.io/salemax/slo-demo-service`,
tag = the chart's `appVersion`, a commit SHA), so nothing needs to be built:

```sh
helm install demo deploy/helm/slo-demo-service --wait
kubectl get pods -l app.kubernetes.io/instance=demo   # 1/1 Running
```

To run a different published image, pass its commit SHA ad hoc
(`--set image.tag=<sha>`) or bump `appVersion` in `Chart.yaml` by PR. To run
uncommitted code, build locally and point the chart at the local image:

```sh
docker build -t slo-demo-service:dev service/
kind load docker-image slo-demo-service:dev
helm install demo deploy/helm/slo-demo-service --wait \
  --set image.repository=slo-demo-service --set image.tag=dev
```

## Reaching the APIs

```sh
# Public API, through the Service
kubectl port-forward svc/demo-slo-demo-service 8080:8080
curl -s localhost:8080/api/fast          # {"status":"ok"}

# Admin API (fault injection): loopback-only inside the pod, not in the Service
kubectl port-forward deploy/demo-slo-demo-service 8081:8081
curl -s localhost:8081/admin/faults      # {"rules":[]}
```

The chart sets `ADMIN_ADDR=127.0.0.1:8081`, overriding the image's
`0.0.0.0:8081`. The admin API has no authentication, and any pod in the cluster
can reach a pod-IP port, so loopback is the protection: `kubectl port-forward`
connects inside the pod's network namespace and needs cluster permissions. It
is deliberately not a value.

## Values

| key | default | note |
|---|---|---|
| `image.repository` | `ghcr.io/salemax/slo-demo-service` | public multi-arch image (amd64, arm64) |
| `image.tag` | `""` | empty = chart `appVersion` (`10b088d`, a commit SHA) |
| `image.pullPolicy` | `IfNotPresent` | SHA tags are immutable; also keeps `kind load` images from being pulled |
| `replicaCount` | `1` | placeholder; fault rules are per pod |
| `service.type` / `service.port` | `ClusterIP` / `8080` | |
| `resources` | 50m / 32Mi request, 64Mi limit | placeholder, untuned |
| `probes.liveness`, `probes.readiness` | see `values.yaml` | placeholder timings on `/healthz` |
| `podAnnotations`, `nodeSelector`, `tolerations`, `affinity` | empty | |
