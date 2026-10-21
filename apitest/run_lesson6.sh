#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
mkdir -p .tools
export BUILDX_CONFIG="${BUILDX_CONFIG:-$PWD/.tools/lesson6-buildx}"
export PIP_CACHE_DIR="${PIP_CACHE_DIR:-$PWD/.tools/pip-cache}"
export PYTHONPYCACHEPREFIX="${PYTHONPYCACHEPREFIX:-$PWD/.tools/python-cache}"
# New local test-only credentials, separate from the user's application .env.
if [ ! -f .tools/lesson6-test.env ]; then
  python3 - <<'PY'
from pathlib import Path
import secrets
p=Path('.tools/lesson6-test.env')
p.write_text('\n'.join(k+'='+secrets.token_hex(24) for k in ('APITEST_PASSWORD','APITEST_SESSION_SECRET','APITEST_MCP_TOKEN'))+'\n')
p.chmod(0o600)
PY
fi
set -a
. ./.tools/lesson6-test.env
set +a
export APITEST_BASE_URL=http://127.0.0.1:18086
export APITEST_DB_HOST=127.0.0.1 APITEST_DB_PORT=13366 APITEST_DB_NAME=campusclaw_test_lesson6 APITEST_DB_USER=apitest
export APITEST_ALLOW_WRITES=1 APITEST_MOCK_STATS=http://127.0.0.1:18767/stats
export APITEST_MCP_BASE_URL=http://mcp-demo:8090
python3 -m venv .venv
.venv/bin/python -m pip install -r apitest/requirements.txt
if [ "${APITEST_CACHED_IMAGES:-0}" = "1" ]; then
  sh apitest/cached-images-lesson6.sh
else
  docker compose --env-file .tools/lesson6-test.env -f apitest/compose.lesson6.yaml -p campusclaw-lesson6 up --build -d --wait
fi
# Recreating API can change its IP; reload nginx's static upstream resolution.
docker compose --env-file .tools/lesson6-test.env -f apitest/compose.lesson6.yaml -p campusclaw-lesson6 restart web
.venv/bin/python apitest/run_all.py
.venv/bin/python apitest/migration_check.py --before
docker compose --env-file .tools/lesson6-test.env -f apitest/compose.lesson6.yaml -p campusclaw-lesson6 restart api
.venv/bin/python - <<'PY'
import time,urllib.request
for _ in range(60):
    try:
        with urllib.request.urlopen('http://127.0.0.1:18086/health',timeout=2) as response:
            if response.status==200:break
    except Exception:time.sleep(.5)
else:raise RuntimeError('API restart failed')
PY
.venv/bin/python apitest/migration_check.py
if [ "${APITEST_BROWSER:-0}" = "1" ]; then
  export PLAYWRIGHT_BROWSERS_PATH="$PWD/.tools/playwright"
  .venv/bin/python -m playwright install chromium
  .venv/bin/python apitest/browser_check.py
fi
