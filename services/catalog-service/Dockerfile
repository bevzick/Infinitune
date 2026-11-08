FROM golang:1.25-alpine AS source

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

FROM source AS build

RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/catalog-service \
    ./cmd/server

FROM scratch

COPY --from=build /out/catalog-service /catalog-service

USER 65532:65532
EXPOSE 8080 9090
HEALTHCHECK --interval=10s --timeout=5s --start-period=10s --retries=5 \
    CMD ["/catalog-service", "healthcheck"]

ENTRYPOINT ["/catalog-service"]
