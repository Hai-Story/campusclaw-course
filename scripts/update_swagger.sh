#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
npm install --prefix .tools/docs --ignore-scripts --no-audit --no-fund swagger-ui-dist@5.33.2
for file in swagger-ui-bundle.js swagger-ui.css LICENSE; do
  cp ".tools/docs/node_modules/swagger-ui-dist/$file" "backend/internal/openapi/static/$file"
done
