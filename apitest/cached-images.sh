#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
docker run --rm -v "$PWD:/workspace" -w /workspace/backend -e GOCACHE=/workspace/.tools/go-cache -e GOPATH=/workspace/.tools/go-path golang:1.22-alpine sh -c 'CGO_ENABLED=0 go build -o .tools/lesson5-api ./cmd/server'
npm --prefix frontend run build
mkdir -p .tools/lesson5-image
cp backend/.tools/lesson5-api .tools/lesson5-image/campusclaw
DOCKER_BUILDKIT=0 docker build -f apitest/Dockerfile.api -t campusclaw-lesson5-api .tools/lesson5-image
mkdir -p .tools/lesson5-web-image
cp -R frontend/dist .tools/lesson5-web-image/
cp frontend/nginx.conf .tools/lesson5-web-image/nginx.conf
DOCKER_BUILDKIT=0 docker build -f apitest/Dockerfile.web -t campusclaw-lesson5-web .tools/lesson5-web-image
docker compose --env-file .tools/lesson5.env -f apitest/compose.yaml -p campusclaw-lesson5 up --no-build -d --wait
