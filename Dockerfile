# syntax=docker/dockerfile:1

# Multi-stage Linux container for the EOC server (SPEC §3, §19.3, P8).
# Node stage builds the Astro/Svelte dashboard → web/dist.
# Go stage builds the static eoc binary.
# Distroless runtime carries both the binary and web/dist for single-origin serving.
# Linux CI/Docker is the authority on "does it build".

# ---- web build (Node) ----
FROM node:25 AS web-build
WORKDIR /src/web

# Cache npm deps
COPY web/package*.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

# ---- go build ----
FROM golang:1.24 AS go-build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# Static binary for distroless
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/eoc ./cmd/eoc

# ---- runtime (distroless) ----
FROM gcr.io/distroless/static-debian12:nonroot

# Binary
COPY --from=go-build /out/eoc /eoc

# Static dashboard (built by Node stage)
COPY --from=web-build /src/web/dist /web/dist

# Default for single-origin serving (P6)
ENV WEB_DIR=/web/dist

USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/eoc"]
