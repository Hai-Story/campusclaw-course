#!/bin/sh
# Compatibility entrypoint: the current API includes lesson-six MCP endpoints.
set -eu
exec sh "$(dirname "$0")/run_lesson6.sh"
