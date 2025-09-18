FROM golang:1.24-bookworm AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG VERSION="dev"
RUN apt-get update && apt-get install git
RUN go build -v -ldflags "-X main.version=${VERSION}" ./cmd/firecore

FROM ubuntu:24.04

ARG TARGETPLATFORM

# gettext-base is needed for envsubst
RUN apt-get update && apt-get -y install ca-certificates htop iotop sysstat strace lsof curl jq tzdata file gettext-base

RUN mkdir -p /app/ && \
    export repository="https://github.com/grpc-ecosystem/grpc-health-probe/releases/download" && \
    export version="v0.4.12" && \
    curl --fail-with-body -Lo /app/grpc_health_probe "$repository/$version/grpc_health_probe-$(echo $TARGETPLATFORM | sed 's|/|-|')" && \
    chmod +x /app/grpc_health_probe

WORKDIR /app

COPY --from=build /app/fireeth /app/fireeth

ENV PATH="$PATH:/app"

#COPY docker/motd /etc/motd
#COPY docker/motd_reader /etc/motd_reader
#COPY docker/99-firehose-core.sh /etc/profile.d/
#COPY docker/scripts/ /app/
RUN chmod +x /app/reader-*
RUN echo ". /etc/profile.d/99-firehose-core.sh" > /root/.bash_aliases

ENTRYPOINT [ "/app/firecore" ]
