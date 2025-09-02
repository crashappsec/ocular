# Copyright (C) 2025 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

FROM golang:1.25.0-alpine@sha256:f18a072054848d87a8077455f0ac8a25886f2397f88bfdd222d6fafbb5bba440 AS builder

WORKDIR /app

ARG LDFLAGS="-w -s"
ARG COMMAND="api-server"

COPY go.mod go.sum /app/

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download


COPY /cmd/${COMMAND}/ /app/cmd/${COMMAND}
COPY /internal /app/internal
COPY /pkg /app/pkg

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go build -ldflags="${LDFLAGS}" -o /app/entrypoint /app/cmd/${COMMAND}/main.go

FROM alpine:3.22@sha256:4bcff63911fcb4448bd4fdacec207030997caf25e9bea4045fa6c8c44de311d1

COPY --from=builder /app/entrypoint /bin/entrypoint

LABEL org.opencontainers.image.source="https://github.com/crashappsec/ocular"

ENTRYPOINT ["/bin/entrypoint"]
