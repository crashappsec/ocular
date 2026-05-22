# Copyright (C) 2025-2026 Crash Override, Inc.
#
# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the FSF, either version 3 of the License, or (at your option) any later version.
# See the LICENSE file in the root of this repository for full license text or
# visit: <https://www.gnu.org/licenses/gpl-3.0.html>.

ARG BUILDPLATFORM

FROM --platform=${BUILDPLATFORM} golang:1.26@sha256:6df14f4a4bc9d979a3721f488981e0d1b318006377e473ed23d026796f5f4c0a AS builder

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

FROM gcr.io/distroless/static:nonroot@sha256:963fa6c544fe5ce420f1f54fb88b6fb01479f054c8056d0f74cc2c6000df5240

WORKDIR /
COPY --from=builder /workspace/entrypoint .
USER 65532:65532

ENTRYPOINT ["/entrypoint"]
