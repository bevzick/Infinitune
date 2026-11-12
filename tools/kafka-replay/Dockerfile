FROM golang:1.25-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/kafka-replay .

FROM scratch
COPY --from=build /out/kafka-replay /kafka-replay
USER 65532:65532
ENTRYPOINT ["/kafka-replay"]
