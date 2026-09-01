# syntax=docker/dockerfile:1
FROM golang:1.25-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# distroless/static has no shell, no package manager, and runs as the
# nonroot (uid 65532) user by default — nothing left to harden.
FROM gcr.io/distroless/static-debian12:nonroot AS final
COPY --from=build /out/api /api

EXPOSE 8080 9090
ENTRYPOINT ["/api"]
