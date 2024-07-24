FROM --platform=$BUILDPLATFORM golang:1.22.5 AS builder

WORKDIR /app
COPY . /app
ARG MODE=devel

RUN make clean $MODE

FROM ubuntu:24.04

COPY --from=builder /app/build/gtexporter /usr/local/bin

RUN apt-get update && \
    apt-get -y dist-upgrade && \
    apt-get clean && \
    rm -r /var/lib/apt/lists/* && \
    groupadd -r gtexporter && \
    useradd -r -g gtexporter gtexporter

USER gtexporter

ENTRYPOINT ["gtexporter"]
