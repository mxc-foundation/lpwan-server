#!/usr/bin/env bash

PROTOBUF_PATH=`go list -f '{{ .Dir }}' github.com/golang/protobuf/ptypes`

protoc -I=. -I=../.. -I=${PROTOBUF_PATH} --go_out=paths=source_relative,plugins=grpc:. geo.proto