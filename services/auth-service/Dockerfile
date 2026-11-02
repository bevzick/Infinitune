FROM golang:1.25-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/auth-service \
    ./cmd/server

FROM scratch

COPY --from=build /out/auth-service /auth-service

USER 65532:65532
EXPOSE 8080

ENTRYPOINT ["/auth-service"]
