FROM golang:1.26-bookworm AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG VERSION="dev"
RUN apt-get update && apt-get install -y git
RUN go build -v -ldflags "-X main.version=${VERSION}" ./cmd/fireeth

FROM ubuntu:24.04

ARG TARGETPLATFORM

# gettext-base is needed for envsubst
RUN apt-get update && apt-get -y install ca-certificates htop iotop sysstat strace lsof curl jq tzdata file gettext-base

RUN mkdir -p /app/ && \
    export repository="https://github.com/grpc-ecosystem/grpc-health-probe/releases/download" && \
    export version=$(curl -s https://api.github.com/repos/grpc-ecosystem/grpc-health-probe/releases/latest | jq -r .tag_name) && \
    curl --fail-with-body -Lo /app/grpc_health_probe "$repository/$version/grpc_health_probe-$(echo $TARGETPLATFORM | sed 's|/|-|')" && \
    chmod +x /app/grpc_health_probe

WORKDIR /app

COPY --from=build /app/fireeth /app/fireeth

ENV PATH="$PATH:/app"

COPY docker/motd /etc/motd
COPY docker/motd_reader /etc/motd_reader
COPY docker/99-firehose-ethereum.sh /etc/profile.d/
COPY docker/scripts/ /app/
RUN chmod +x /app/reader-*
RUN echo ". /etc/profile.d/99-firehose-ethereum.sh" > /root/.bash_aliases

ENTRYPOINT [ "/app/fireeth" ]
