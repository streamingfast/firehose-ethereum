ARG COREVERSION="latest"

FROM golang:1.24.2-bookworm AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

# to get buildinfo in golang
RUN apt-get update && apt-get -y install git
ARG VERSION="dev"
RUN go build -v -ldflags "-X main.version=${VERSION}" ./cmd/fireeth

####

FROM ghcr.io/streamingfast/firehose-core:${COREVERSION} as core

####

FROM ubuntu:24.04

ENV PATH "$PATH:/app"

COPY tools/fireeth/motd_generic /etc/motd
COPY tools/fireeth/99-fireeth.sh /etc/profile.d/
RUN echo ". /etc/profile.d/99-fireeth.sh" > /root/.bash_aliases

RUN apt-get update && apt-get -y install ca-certificates htop iotop sysstat strace lsof curl jq tzdata

RUN mkdir -p /app/ && curl -Lo /app/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.12/grpc_health_probe-linux-amd64 && chmod +x /app/grpc_health_probe

WORKDIR /app

COPY --from=build /app/fireeth /app/fireeth
COPY --from=core /app/firecore /app/firecore

ENTRYPOINT ["/app/fireeth"]
