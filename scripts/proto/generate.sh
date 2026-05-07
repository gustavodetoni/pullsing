#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BUF_VERSION="${BUF_VERSION:-v1.55.1}"

cd "${ROOT_DIR}"

mkdir -p proto/gen/go

find proto/gen/go -type f \( -name '*.pb.go' -o -name '*_grpc.pb.go' \) -delete

go run "github.com/bufbuild/buf/cmd/buf@${BUF_VERSION}" generate
