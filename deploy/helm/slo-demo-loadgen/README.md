# slo-demo-loadgen Helm chart

Deploys the load generator from [`loadgen/`](../../../loadgen/): one
Deployment, no Service. Install it after the
[`slo-demo-service`](../slo-demo-service/) chart (release name `demo`).

Loadgen listens on nothing: no HTTP server, no `/metrics`, no health endpoint
(checked in `loadgen/cmd/loadgen/main.go`; it only runs the client loop and
logs). So there is no Service, no container port and no probes. Its traffic is
measured on the service side (`http_requests_total`), which is the point of the
SLI. Its own view is the log.

## Install on a local kind cluster

There is no published image yet, so build and load it. The service must be
installed first, otherwise loadgen logs connection errors until it exists.

```sh
docker build -t slo-demo-service:dev service/
docker build -t slo-demo-loadgen:dev loadgen/
kind load docker-image slo-demo-service:dev slo-demo-loadgen:dev
helm install demo deploy/helm/slo-demo-service --wait
helm install loadgen deploy/helm/slo-demo-loadgen --wait
kubectl logs deploy/loadgen-slo-demo-loadgen -f
```

Release name `loadgen` is used so the Deployment is not called
`demo-slo-demo-loadgen`; any name works.

## Target

`LOADGEN_TARGET` is the only variable the chart sets (the compose stack does
the same). The binary's default, `http://localhost:8080`, is the loadgen pod
itself, so the chart always sets it. The default value,
`http://demo-slo-demo-service:8080`, is the Service DNS name of the service
chart installed as release `demo` in the same namespace. For another release or
namespace:

```sh
helm install loadgen deploy/helm/slo-demo-loadgen \
  --set target=http://<release>-slo-demo-service.<namespace>:8080
```

## Rate, mix, timeout and in-flight cap are not values

Their defaults (20 req/s, `/api/fast=80,/api/slow=20`, 10 s timeout, 1000
in-flight) are owner placeholders, and the timeout is decision D5. They live
only in the Go code, and neither the compose stack nor this chart sets them, so
there is one place to change them. For a one-off experiment use
`kubectl set env deploy/<name> LOADGEN_RATE=50` (see
[`loadgen/README.md`](../../../loadgen/README.md) for the variables). Note that
a lower timeout creates `499` events, which are bad events under D1.

## Values

| key | default | note |
|---|---|---|
| `image.repository` | `slo-demo-loadgen` | no registry baked in |
| `image.tag` | `dev` | |
| `image.pullPolicy` | `IfNotPresent` | needed for `kind load` images |
| `replicaCount` | `1` | each replica adds its own full offered load |
| `target` | `http://demo-slo-demo-service:8080` | becomes `LOADGEN_TARGET` |
| `resources` | 25m / 32Mi request, 128Mi limit | placeholder; the limit is the compose `mem_limit: 128m`, and the worst case scales with the in-flight cap |
| `podAnnotations`, `nodeSelector`, `tolerations`, `affinity` | empty | |

The pod runs as the distroless nonroot user (65532) with a read-only root
filesystem, no capabilities and no service account token.
