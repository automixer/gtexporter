FROM golang:1.21.6-bookworm as builder

WORKDIR /app
COPY . /app

RUN make clean docker

FROM ubuntu:24.04

COPY --from=builder /app/build/* /usr/local/bin

ENTRYPOINT ["gtexporter"]
