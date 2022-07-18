FROM golang:1.18-alpine3.15 AS build

ENV CGO_ENABLED 0
ENV DOCKER_BUILDKIT 1
ENV GOOS=linux
ENV GOARCH=amd64

RUN mkdir -p /tmp/sfeth/
COPY . /tmp/sfeth/
RUN cd /tmp/sfeth && GOOS=linux GOARCH=amd64 go build ./cmd/sfeth

FROM ubuntu:20.04

RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
       apt-get -y install -y \
       ca-certificates libssl1.1 vim htop iotop sysstat \
       dstat strace lsof curl jq tzdata && \
       rm -rf /var/cache/apt /var/lib/apt/lists/*

RUN rm /etc/localtime && ln -snf /usr/share/zoneinfo/America/Montreal /etc/localtime && dpkg-reconfigure -f noninteractive tzdata

RUN mkdir /tmp/wasmer-install && cd /tmp/wasmer-install && \
       curl -L https://github.com/wasmerio/wasmer/releases/download/2.2.1/wasmer-linux-amd64.tar.gz | tar xzf - && \
       mv lib/libwasmer.a lib/libwasmer.so /usr/lib/ && cd / && rm -rf /tmp/wasmer-install

COPY --from=build /tmp/sfeth/sfeth /sfeth

ADD /sfeth /app/sfeth

COPY tools/sfeth/motd_generic /etc/
COPY tools/sfeth/99-sfeth-generic.sh /etc/profile.d/

ENTRYPOINT ["/app/sfeth"]
