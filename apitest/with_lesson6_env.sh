#!/bin/sh
# Run an acceptance command with only the dedicated lesson-six test configuration.
set -eu
cd "$(dirname "$0")/.."
set -a
. ./.tools/lesson6-test.env
set +a
export APITEST_BASE_URL=http://127.0.0.1:18086
export APITEST_DB_HOST=127.0.0.1 APITEST_DB_PORT=13366 APITEST_DB_NAME=campusclaw_test_lesson6 APITEST_DB_USER=apitest
export APITEST_ALLOW_WRITES=1 APITEST_MOCK_STATS=http://127.0.0.1:18767/stats
export APITEST_MCP_BASE_URL=http://mcp-demo:8090
export BUILDX_CONFIG="$PWD/.tools/lesson6-buildx" PYTHONPYCACHEPREFIX="$PWD/.tools/python-cache"
exec "$@"
