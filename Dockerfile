# syntax=docker/dockerfile:1.7

# Default as a fallback; will be overridden by build-arg from CI
ARG GO_VERSION=1.24.2

# Builder
FROM golang:${GO_VERSION} AS builder
WORKDIR /src

# Allow auto toolchain upgrades if go.mod requires a newer patch
ENV GOTOOLCHAIN=auto

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
ARG MODE=devel
RUN make clean $MODE

# Runtime
FROM ubuntu:24.04
WORKDIR /app
COPY --from=builder /src/build/gtexporter /usr/local/bin/gtexporter
ENTRYPOINT ["/usr/local/bin/gtexporter"]
