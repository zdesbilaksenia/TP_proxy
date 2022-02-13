# syntax=docker/dockerfile:1
FROM golang:1.17-alpine
WORKDIR /app
COPY go.mod ./

RUN go mod download

COPY *.go ./

RUN go get proxy

RUN go build -o /proxy
CMD ["/proxy"]