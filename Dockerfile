FROM --platform=$BUILDPLATFORM golang:1.23.2 AS builder

WORKDIR /app
COPY . /app
ARG MODE=devel

RUN make clean $MODE

FROM ubuntu:24.04

COPY --from=builder /app/build/gtexporter /usr/local/bin

RUN groupadd -r gtexporter && \
    useradd -r -g gtexporter gtexporter

USER gtexporter

ENTRYPOINT ["gtexporter"]
