#!/bin/bash

PROTO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
echo "$PROTO_ROOT"

# Generate for infra/protocol
protoc \
  --go_out="$PROTO_ROOT/infra/pb/protocol" \
  --go-grpc_out="$PROTO_ROOT/infra/pb/protocol" \
  --go_opt=module=github.com/phuhao00/dafuweng/infra/pb/protocol \
  --go-grpc_opt=module=github.com/phuhao00/dafuweng/infra/pb/protocol \
  --proto_path="$PROTO_ROOT" \
  --proto_path="$PROTO_ROOT/infra/protocol" \
  "$PROTO_ROOT"/infra/protocol/*.proto

# Generate for infra/model
protoc \
  --go_out="$PROTO_ROOT/infra/pb/model" \
  --go-grpc_out="$PROTO_ROOT/infra/pb/model" \
  --go_opt=module=github.com/phuhao00/dafuweng/infra/pb/model \
  --go-grpc_opt=module=github.com/phuhao00/dafuweng/infra/pb/model \
  --proto_path="$PROTO_ROOT" \
  --proto_path="$PROTO_ROOT/infra/model" \
  --proto_path="$PROTO_ROOT/infra/protocol" \
  "$PROTO_ROOT"/infra/model/*.proto