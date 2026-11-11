# Go 1.27.4 rejects the libwasmtime.a bundled with wasmtime-go (disallowed LTO section ".llvmbc").
FROM golang:1.27.3-bookworm AS build
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

# Ubuntu ships no fix yet for the apt CVE-2026-109518, nothing runs apt on this image so drop it
RUN dpkg --purge --force-remove-protected --force-remove-essential apt libapt-pkg6.0t64 && rm -f /var/lib/dpkg/status-old

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
