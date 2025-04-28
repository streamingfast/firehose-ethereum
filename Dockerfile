ARG COREVERSION="latest"

FROM golang:1.24-bookworm AS build
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./

RUN apt-get update && apt-get -y install git
ARG VERSION="dev"
RUN go build -v -ldflags "-X main.version=${VERSION}" ./cmd/fireeth

FROM ghcr.io/streamingfast/firehose-core:${COREVERSION}

COPY --from=build /app/fireeth /app/fireeth

ENTRYPOINT ["/app/fireeth"]
