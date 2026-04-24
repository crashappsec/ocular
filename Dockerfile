# Copyright (C) 2025-2026 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

ARG BUILDPLATFORM

FROM --platform=${BUILDPLATFORM} golang:1.26@sha256:1e598ea5752ae26c093b746fd73c5095af97d6f2d679c43e83e0eac484a33dc3 AS builder

ARG TARGETOS
ARG TARGETARCH
ARG LDFLAGS="-w -s"
ARG COMMAND="controller"

ARG GOFLAGS=""
ENV GOFLAGS="${GOFLAGS} -trimpath"

WORKDIR /workspace

COPY go.mod go.sum .

RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY cmd/$COMMAND cmd/$COMMAND
COPY api/ api/
COPY internal/ internal/
COPY pkg/ pkg/

RUN --mount=type=cache,target=/go/pkg/mod CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} \
    go build -ldflags="${LDFLAGS}" -o entrypoint cmd/$COMMAND/main.go

FROM gcr.io/distroless/static:nonroot@sha256:e3f945647ffb95b5839c07038d64f9811adf17308b9121d8a2b87b6a22a80a39

WORKDIR /
COPY --from=builder /workspace/entrypoint .
USER 65532:65532

ENTRYPOINT ["/entrypoint"]
