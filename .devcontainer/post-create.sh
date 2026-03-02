#!/bin/bash
set -e

TRIVY_VERSION=0.69.2

echo "==> Installing Trivy v${TRIVY_VERSION}..."
curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sudo sh -s -- -b /usr/local/bin "v${TRIVY_VERSION}"

echo "==> Installing Go dependencies..."
go mod download

echo "==> Installing frontend dependencies..."
cd web && npm ci && cd ..

echo "==> Done! Run 'make run' to start."
