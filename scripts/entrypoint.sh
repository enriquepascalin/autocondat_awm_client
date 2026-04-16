#!/bin/sh
set -e

# AWM CLI Container Entrypoint
# This script runs when the CLI is executed inside a Docker container.
# It ensures any required runtime setup is performed before starting the agent.

echo "=== AWM CLI Entrypoint ==="

# ----------------------------------------------------------------------
# 1. Verify configuration exists
# ----------------------------------------------------------------------
if [ ! -f "/app/configs/agent.yaml" ]; then
    echo "ERROR: Configuration file not found at /app/configs/agent.yaml"
    echo "       Please mount a configuration file or set AWM_CONFIG environment variable."
    exit 1
fi

# ----------------------------------------------------------------------
# 2. Optionally wait for orchestrator to be reachable
# ----------------------------------------------------------------------
if [ -n "$AWM_WAIT_FOR_ORCHESTRATOR" ]; then
    echo ">>> Waiting for orchestrator at $AWM_WAIT_FOR_ORCHESTRATOR..."
    while ! nc -z $(echo "$AWM_WAIT_FOR_ORCHESTRATOR" | sed 's/:/ /'); do
        sleep 2
    done
    echo ">>> Orchestrator is reachable."
fi

# ----------------------------------------------------------------------
# 3. Execute the CLI binary
# ----------------------------------------------------------------------
echo ">>> Starting AWM CLI agent..."
exec /app/awm-cli "$@"