FROM ubuntu:20.04 as build
WORKDIR /build

ARG GO_VERSION=1.18.8

RUN apt-get update && apt-get install curl gcc -y
RUN curl -L https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz -o go${GO_VERSION}.linux-amd64.tar.gz && \
       tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz

ENV PATH "$PATH:/usr/local/go/bin"

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG BUILD_VERSION=local
ARG BUILD_COMMIT=dev
ARG BUILD_DATE=1970-01-01T00:00Z

RUN go build -v -ldflags "-X main.version=${BUILD_VERSION} -X main.commit=${BUILD_COMMIT} -X main.date=${BUILD_DATE}" -o ./fireeth ./cmd/fireeth

FROM ubuntu:20.04 

RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
       apt-get -y install -y \
       ca-certificates libssl1.1 vim htop iotop sysstat \
       dstat strace lsof curl jq tzdata && \
       rm -rf /var/cache/apt /var/lib/apt/lists/*

RUN rm /etc/localtime && ln -snf /usr/share/zoneinfo/America/Montreal /etc/localtime && dpkg-reconfigure -f noninteractive tzdata

COPY tools/fireeth/motd_generic /etc/motd
COPY tools/fireeth/99-fireeth.sh /etc/profile.d/

COPY --from=build /build/fireeth /app/fireeth

# On SSH connection, /root/.bashrc is invoked which invokes '/root/.bash_aliases' if existing,
# so we hijack the file to "execute" our specialized bash script
RUN echo ". /etc/profile.d/99-fireeth.sh" > /root/.bash_aliases

ENV PATH "$PATH:/app"

ENTRYPOINT ["/app/fireeth"]
