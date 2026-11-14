FROM golang:1.25-alpine AS source

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

FROM source AS build

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/playlist-service \
    ./cmd/server

FROM scratch

COPY --from=source /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/playlist-service /playlist-service

USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
    CMD ["/playlist-service", "healthcheck"]

ENTRYPOINT ["/playlist-service"]
