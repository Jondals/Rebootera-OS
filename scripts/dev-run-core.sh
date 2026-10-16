#!/bin/bash
# * Arranca rebootera-core en WSL para desarrollo, sirviendo la interfaz de
# * ui/dist. Imprime la URL de acceso con el token. No arranca el agente: las
# * acciones de energía responderán que el servicio del sistema no responde.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export GOCACHE=/root/.cache/go-build GOFLAGS=-buildvcs=false

mkdir -p /tmp/rebootera-dev
cd "$ROOT/core"
CGO_ENABLED=0 go build -o /tmp/rebootera-dev/rebootera-core ./cmd/rebootera-core

/tmp/rebootera-dev/rebootera-core \
	-listen 127.0.0.1:7080 \
	-ui "$ROOT/ui/dist" \
	-token-file /tmp/rebootera-dev/ui-token &
PID=$!
for _ in $(seq 1 50); do
	[ -s /tmp/rebootera-dev/ui-token ] && break
	sleep 0.1
done
echo "URL: http://127.0.0.1:7080/auth?token=$(cat /tmp/rebootera-dev/ui-token)"
wait "$PID"
