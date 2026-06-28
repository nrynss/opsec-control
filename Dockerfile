# syntax=docker/dockerfile:1

# Linux container for the EOC server (SPEC §3, §19.3). Linux CI/Docker is the
# authority on "does it build" — a case-mismatched import or bad embed fails
# here even when it builds on macOS/Windows.

# ---- build ----
FROM golang:1.24 AS build
WORKDIR /src

# Cache module downloads. Add `go.sum` to this COPY once dependencies exist.
COPY go.mod ./
RUN go mod download

COPY . .
# Static binary so it runs on the distroless/static base.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/eoc ./cmd/eoc

# ---- runtime ----
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/eoc /eoc
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/eoc"]
