#!/bin/sh
# Usage: deploy/kind/kind.sh up|down
# A POSIX script rather than a Makefile: "create the cluster only if it does not
# exist" is a shell conditional, and a script with two subcommands is easier to
# read top to bottom than make targets plus embedded shell.
set -eu

CLUSTER=slo-demo
CTX="kind-$CLUSTER"
ROOT=$(cd "$(dirname "$0")/../.." && pwd)

case "${1:-}" in
up)
  # The Grafana admin password comes from the environment, never from the repo.
  # Checked first so a missing password fails before anything is created.
  : "${GRAFANA_ADMIN_PASSWORD:?set GRAFANA_ADMIN_PASSWORD (e.g. export GRAFANA_ADMIN_PASSWORD=...) before kind.sh up}"

  if kind get clusters | grep -qx "$CLUSTER"; then
    echo "cluster $CLUSTER already exists, reusing it"
  else
    kind create cluster --config "$ROOT/deploy/kind/cluster.yaml"
  fi
  # Image tag = commit SHA, so `kubectl get deploy -o wide` shows which commit
  # runs. A fixed tag with pullPolicy IfNotPresent would keep stale pods after a
  # rebuild; a new tag changes the pod template, so Helm rolls the Deployment.
  # Uncommitted changes get a unique suffix, otherwise a rebuilt dirty image
  # would reuse the tag and hit the same stale-pod problem.
  TAG=$(git -C "$ROOT" rev-parse --short HEAD)
  if [ -n "$(git -C "$ROOT" status --porcelain)" ]; then
    TAG="$TAG-dirty-$(date +%s)"
  fi
  echo "image tag: $TAG"

  for app in service loadgen; do
    docker build -t "slo-demo-$app:$TAG" "$ROOT/$app"
    kind load docker-image "slo-demo-$app:$TAG" --name "$CLUSTER"
  done
  # Explicit --kube-context: never deploy into whatever cluster is current.
  # The service goes first: the loadgen and Prometheus both target it.
  helm upgrade --install demo "$ROOT/deploy/helm/slo-demo-service" \
    --set image.tag="$TAG" --kube-context "$CTX" --wait
  helm upgrade --install loadgen "$ROOT/deploy/helm/slo-demo-loadgen" \
    --set image.tag="$TAG" --kube-context "$CTX" --wait

  # Rules and dashboard are loaded from their original files in monitoring/, so
  # the promtool-tested files stay the single copy. `create --dry-run | apply`
  # makes the command idempotent. Prometheus sees changed rules only after a
  # restart (the config reloader is off, see prometheus-values.yaml).
  kubectl --context "$CTX" create configmap slo-demo-prometheus-rules \
    --from-file="$ROOT/monitoring/prometheus/rules" --dry-run=client -o yaml |
    kubectl --context "$CTX" apply -f -
  kubectl --context "$CTX" create configmap slo-demo-dashboards \
    --from-file="$ROOT/monitoring/grafana/provisioning/dashboards/json/slo-demo.json" \
    --dry-run=client -o yaml | kubectl --context "$CTX" apply -f -
  # The password goes in on stdin: --from-literal would put it on the kubectl
  # command line, where `ps` shows it. printf is a shell builtin, so it never
  # appears in a process list either.
  printf '%s' "$GRAFANA_ADMIN_PASSWORD" |
    kubectl --context "$CTX" create secret generic grafana-admin \
      --from-literal=admin-user=admin --from-file=admin-password=/dev/stdin \
      --dry-run=client -o yaml | kubectl --context "$CTX" apply -f -

  # Chart versions are pinned (checked against the chart repo indexes on
  # 2026-11-11). The Grafana chart lives in grafana-community; the old
  # grafana/helm-charts copy is deprecated.
  helm upgrade --install prometheus prometheus \
    --repo https://prometheus-community.github.io/helm-charts --version 29.47.1 \
    -f "$ROOT/deploy/kind/prometheus-values.yaml" --kube-context "$CTX" --wait
  helm upgrade --install grafana grafana \
    --repo https://grafana-community.github.io/helm-charts --version 14.4.1 \
    -f "$ROOT/deploy/kind/grafana-values.yaml" --kube-context "$CTX" --wait
  ;;
down)
  kind delete cluster --name "$CLUSTER"
  ;;
*)
  echo "usage: $0 up|down" >&2
  exit 2
  ;;
esac
