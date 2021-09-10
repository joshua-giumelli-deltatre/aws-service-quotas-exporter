FROM golang:alpine as builder

RUN adduser -D -g '' exporter

RUN apk update

WORKDIR /exporter

COPY . /exporter

RUN GOOS=linux go build -o /go/bin/exporter /exporter/cmd

FROM alpine

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /go/bin/exporter /app/exporter

USER exporter
WORKDIR /app

ENTRYPOINT ["exporter"]