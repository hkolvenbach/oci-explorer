# Frontend build stage
FROM node:22.14-alpine AS frontend

WORKDIR /app/web

COPY web/package.json web/package-lock.json* ./
RUN npm ci

COPY web/ ./
RUN npm run build

# Go build stage
FROM --platform=$BUILDPLATFORM golang:1.26.0-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Copy frontend build output
COPY --from=frontend /app/web/dist ./web/dist

# Build the binary with deterministic flags
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -buildvcs=false \
    -ldflags="-w -s -X main.Version=${VERSION}" -o /oci-explorer .

# Trivy stage — download pinned release for the target platform
FROM alpine:3.21 AS trivy-dl

ARG TRIVY_VERSION=0.69.2

RUN apk add --no-cache curl
RUN curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh -s -- -b /usr/local/bin v${TRIVY_VERSION}

# Runtime stage — distroless (zero CVEs, no shell, no package manager)
FROM gcr.io/distroless/static-debian12

WORKDIR /app

# Copy binary from builder
COPY --from=builder /oci-explorer .

# Copy trivy binary (Apache 2.0 licensed — https://github.com/aquasecurity/trivy/blob/main/LICENSE)
COPY --from=trivy-dl /usr/local/bin/trivy /usr/local/bin/trivy

# Distroless ships a nonroot user (UID 65532)
USER nonroot:nonroot

# Expose port
EXPOSE 8080

# Run the application
ENTRYPOINT ["/app/oci-explorer"]
