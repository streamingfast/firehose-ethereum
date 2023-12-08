FROM golang:1.21-alpine as build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

ARG VERSION
RUN go build -v -ldflags "-X main.version=${VERSION}" ./cmd/fireeth

####

FROM alpine:edge

ENV PATH "$PATH:/app"

COPY tools/fireeth/motd_generic /etc/motd
COPY tools/fireeth/99-fireeth.sh /etc/profile.d/
RUN echo ". /etc/profile.d/99-fireeth.sh" > /root/.bash_aliases

RUN apk --no-cache add \
        ca-certificates htop iotop sysstat \
        strace lsof curl jq tzdata

RUN mkdir -p /app/ && curl -Lo /app/grpc_health_probe https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/v0.4.12/grpc_health_probe-linux-amd64 && chmod +x /app/grpc_health_probe

WORKDIR /app

COPY --from=build /app/fireeth /app/fireeth

ENTRYPOINT ["/app/fireeth"]
