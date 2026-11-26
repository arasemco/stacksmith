# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build
ENV CGO_ENABLED=0
WORKDIR /src

RUN --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app

COPY --from=build --chown=65532:65532 --link /out/api /app/api

USER nonroot:nonroot
EXPOSE 8080

LABEL org.opencontainers.image.source="https://github.com/arasemco/stacksmith"

ENTRYPOINT ["/app/api", "-addr", ":8080", "-root", "/app/data"]
