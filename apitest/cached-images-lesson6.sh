#!/bin/sh
# Offline acceptance uses existing public base images, never the application's old binaries.
set -eu
cd "$(dirname "$0")/.."
mkdir -p .tools/lesson6-image .tools/lesson6-mcp-image .tools/lesson6-web-image
for module in backend mcp-server; do
  output=campusclaw
  target=./cmd/server
  if [ "$module" = "mcp-server" ]; then output=mcp-demo; target=.; fi
  docker run --rm --network none -v "$PWD:/workspace" -w "/workspace/$module" \
    -e GOCACHE=/workspace/.tools/go-cache-linux -e GOPATH=/workspace/.tools/go-path \
    -e GOPROXY=off -e GOTOOLCHAIN=local golang:1.22-alpine \
    sh -c "CGO_ENABLED=0 go build -trimpath -o /workspace/.tools/lesson6-runtime/$output $target"
done
cp .tools/lesson6-runtime/campusclaw .tools/lesson6-image/campusclaw
cp .tools/lesson6-runtime/mcp-demo .tools/lesson6-mcp-image/mcp-demo
npm --prefix frontend run build
cp -R frontend/dist .tools/lesson6-web-image/
cp frontend/nginx.conf .tools/lesson6-web-image/nginx.conf
DOCKER_BUILDKIT=0 docker build --network none -f apitest/Dockerfile.api -t campusclaw-lesson6-api .tools/lesson6-image
DOCKER_BUILDKIT=0 docker build --network none -f apitest/Dockerfile.mcp-offline -t campusclaw-lesson6-mcp-demo .tools/lesson6-mcp-image
DOCKER_BUILDKIT=0 docker build --network none -f apitest/Dockerfile.web -t campusclaw-lesson6-web .tools/lesson6-web-image
docker compose --env-file .tools/lesson6-test.env -f apitest/compose.lesson6.yaml -p campusclaw-lesson6 up --no-build -d --wait
