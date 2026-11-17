FROM golang:1.23-alpine AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /provider ./cmd/provider
FROM alpine:3.21
RUN adduser -D -u 10001 provider
COPY --from=build /provider /usr/local/bin/provider
USER provider
ENTRYPOINT ["/usr/local/bin/provider"]
