# Backend API image. Built locally by docker compose and by Render in staging
# and production. The frontend has no Dockerfile.

FROM golang:1.25.14-alpine AS build
WORKDIR /src
ENV CGO_ENABLED=0 GOFLAGS=-trimpath

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
RUN go build -ldflags="-s -w" -o /out/api ./cmd/api

# Distroless: no shell, no package manager, runs as an unprivileged user.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/api /app/api
USER nonroot:nonroot
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
  CMD ["/app/api", "healthcheck"]
ENTRYPOINT ["/app/api"]
