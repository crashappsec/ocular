# Copyright (C) 2025 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

FROM golang:1.25@sha256:8305f5fa8ea63c7b5bc85bd223ccc62941f852318ebfbd22f53bbd0b358c07e1 AS builder
ARG TARGETOS
ARG TARGETARCH
ARG LDFLAGS="-w -s"
ARG COMMAND="controller"

ARG GOFLAGS=""
ENV GOFLAGS="${GOFLAGS} -trimpath"

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum

RUN go mod download

COPY cmd/$COMMAND cmd/$COMMAND
COPY api/ api/
COPY internal/ internal/
COPY pkg/ pkg/


RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="${LDFLAGS}" -o entrypoint cmd/$COMMAND/main.go

FROM gcr.io/distroless/static:nonroot@sha256:e8a4044e0b4ae4257efa45fc026c0bc30ad320d43bd4c1a7d5271bd241e386d0

WORKDIR /
COPY --from=builder /workspace/entrypoint .
USER 65532:65532

ENTRYPOINT ["/entrypoint"]
