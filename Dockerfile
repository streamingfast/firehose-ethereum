FROM golang:1.18 as builder
ARG REV
ARG VERSION
#RUN go build -v -ldflags "-X main.Version=$VERSION -X main.Commit=${REV}" -o /sfeth ./cmd/sfeth
RUN pwd && ls
#RUN find /

#FROM ubuntu:20.04
#RUN DEBIAN_FRONTEND=noninteractive apt-get update && \
#	   apt-get -y install -y \
#       ca-certificates libssl1.1 vim htop iotop sysstat \
#       dstat strace lsof curl jq tzdata && \
#       rm -rf /var/cache/apt /var/lib/apt/lists/*
#RUN rm /etc/localtime && ln -snf /usr/share/zoneinfo/America/Montreal /etc/localtime && dpkg-reconfigure -f noninteractive tzdata
#COPY --from=builder /sfeth /app/sfeth
