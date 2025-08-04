# Copyright (C) 2025 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

FROM golang:1.24.5-alpine@sha256:daae04ebad0c21149979cd8e9db38f565ecefd8547cf4a591240dc1972cf1399 AS builder

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

FROM alpine:3.22@sha256:ddf52008bce1be455fe2b22d780b6693259aaf97b16383b6372f4b22dd33ad66

COPY --from=builder /app/entrypoint /bin/entrypoint

LABEL org.opencontainers.image.source="https://github.com/crashappsec/ocular"

ENTRYPOINT ["/bin/entrypoint"]
