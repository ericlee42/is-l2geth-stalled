# syntax=docker/dockerfile:1
FROM golang:1.22.1-alpine as compiler
WORKDIR /app
COPY . .
RUN go install

FROM alpine:3.19.1
COPY --from=compiler /go/bin/* /usr/local/bin/
ENTRYPOINT [ "is-l2geth-stalled" ]
