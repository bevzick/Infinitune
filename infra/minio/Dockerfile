ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.22.2
ARG MINIO_VERSION=RELEASE.2025-10-15T17-29-55Z
ARG MINIO_COMMIT=9e49d5e7a648f00e26f2246f4dc28e6b07f8c84a
ARG MC_VERSION=RELEASE.2025-08-13T08-35-41Z

FROM golang:${GO_VERSION}-alpine AS minio-build

ARG MINIO_VERSION
ARG MINIO_COMMIT
ENV CGO_ENABLED=0 GOBIN=/out

RUN go install -ldflags="-s -w \
    -X github.com/minio/minio/cmd.Version=2025-10-15T17:29:55Z \
    -X github.com/minio/minio/cmd.CopyrightYear=2025 \
    -X github.com/minio/minio/cmd.ReleaseTag=${MINIO_VERSION} \
    -X github.com/minio/minio/cmd.CommitID=${MINIO_COMMIT} \
    -X github.com/minio/minio/cmd.ShortCommitID=9e49d5e7a648" \
    github.com/minio/minio@${MINIO_VERSION}

FROM golang:${GO_VERSION}-alpine AS mc-build

ARG MC_VERSION
ENV CGO_ENABLED=0 GOBIN=/out

RUN go install github.com/minio/mc@${MC_VERSION}

FROM alpine:${ALPINE_VERSION} AS runtime

RUN apk add --no-cache ca-certificates curl && \
    addgroup -S -g 1000 minio && \
    adduser -S -D -H -u 1000 -G minio minio && \
    mkdir -p /data /tmp/.mc && \
    chown -R minio:minio /data /tmp/.mc

COPY --from=minio-build /out/minio /usr/local/bin/minio
COPY --from=mc-build /out/mc /usr/local/bin/mc

ENV MC_CONFIG_DIR=/tmp/.mc

USER minio

FROM runtime AS server

ARG MINIO_VERSION

LABEL org.opencontainers.image.source="https://github.com/minio/minio" \
      org.opencontainers.image.version="${MINIO_VERSION}" \
      org.opencontainers.image.licenses="AGPL-3.0-only"

EXPOSE 9000
VOLUME ["/data"]
ENTRYPOINT ["minio"]

FROM runtime AS client

ARG MC_VERSION

LABEL org.opencontainers.image.source="https://github.com/minio/mc" \
      org.opencontainers.image.version="${MC_VERSION}" \
      org.opencontainers.image.licenses="AGPL-3.0-only"

ENTRYPOINT ["mc"]
