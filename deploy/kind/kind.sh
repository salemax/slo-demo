#!/bin/sh
# Usage: deploy/kind/kind.sh up|down
# A POSIX script rather than a Makefile: "create the cluster only if it does not
# exist" is a shell conditional, and a script with two subcommands is easier to
# read top to bottom than make targets plus embedded shell.
set -eu

CLUSTER=slo-demo
IMAGE=slo-demo-service:dev
ROOT=$(cd "$(dirname "$0")/../.." && pwd)

case "${1:-}" in
up)
  if kind get clusters | grep -qx "$CLUSTER"; then
    echo "cluster $CLUSTER already exists, reusing it"
  else
    kind create cluster --config "$ROOT/deploy/kind/cluster.yaml"
  fi
  docker build -t "$IMAGE" "$ROOT/service"
  kind load docker-image "$IMAGE" --name "$CLUSTER"
  # Explicit --kube-context: never deploy into whatever cluster is current.
  # With a fixed :dev tag and pullPolicy IfNotPresent, a rebuilt image is not
  # picked up by running pods; see the README for `kubectl rollout restart`.
  helm upgrade --install demo "$ROOT/deploy/helm/slo-demo-service" \
    --kube-context "kind-$CLUSTER" --wait
  ;;
down)
  kind delete cluster --name "$CLUSTER"
  ;;
*)
  echo "usage: $0 up|down" >&2
  exit 2
  ;;
esac
