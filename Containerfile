# syntax=docker/dockerfile:1
# Global args
ARG HELM_VERSION=v3.18.3

# Download and verify Helm binary in builder stage
FROM alpine:3.22.0 AS builder

# Define Helm version and architecture
ARG HELM_VERSION
ARG TARGETOS
ARG TARGETARCH
ARG HELM_BASE_URL=https://get.helm.sh
ARG HELM_GITHUB_URL=https://github.com/helm/helm/releases/download/${HELM_VERSION}

# Install only essential packages for downloading and verification
RUN apk add --no-cache \
    curl \
    gnupg \
    tar \
    && rm -rf /var/cache/apk/*

# Create non-root user in builder stage
RUN echo "nonroot:x:65532:65532:nonroot:/tmp:/sbin/nologin" >> /etc/passwd \
    && echo "nonroot:x:65532:" >> /etc/group

# Download Helm binary, checksums, and signature
RUN curl -fsSLO "${HELM_BASE_URL}/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz" \
    && curl -fsSLO "${HELM_BASE_URL}/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz.sha256sum" \
    && curl -fsSLO "${HELM_GITHUB_URL}/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz.asc" \
    && curl -fsSLO "${HELM_BASE_URL}/KEYS"

# Import Helm public keys and verify signature
RUN gpg --import KEYS \
    && gpg --verify "helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz.asc" "helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz"

# Verify checksum
RUN sha256sum -c "helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz.sha256sum"

# Extract Helm binary
RUN tar -xzf "helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz" ${TARGETOS}-${TARGETARCH}/helm \
    && mv ${TARGETOS}-${TARGETARCH}/helm /usr/local/bin/helm \
    && chmod +x /usr/local/bin/helm

# Verify helm binary works
RUN /usr/local/bin/helm version --client

# Final stage - use scratch for absolute minimal attack surface
FROM scratch

# Redeclare version for metadata
ARG HELM_VERSION

# Set metadata
LABEL org.opencontainers.image.title="Helm"
LABEL org.opencontainers.image.description="Secure Helm container"
LABEL org.opencontainers.image.version="${HELM_VERSION#v}"
LABEL org.opencontainers.image.authors="cncf-helm-core-maintainers@lists.cncf.io"
LABEL org.opencontainers.image.documentation="https://helm.sh/docs/install"
LABEL org.opencontainers.image.source="https://github.com/helm/helm/blob/main/Containerfile"
LABEL org.opencontainers.image.licenses="Apache-2.0"

# Copy only the Helm binary from builder stage
COPY --from=builder /usr/local/bin/helm /helm

# Copy minimal required files for user context
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

# Set security context - use nonroot user
USER 65532:65532

# Set working directory
WORKDIR /workspace

# Default command
ENTRYPOINT ["/helm"]
CMD ["--help"]

# Security notes:
# 1. Uses scratch base image - absolutely nothing except what we add
# 2. Multi-stage build to avoid build tools in final image
# 3. Checksum verification of downloaded binary
# 4. Non-root user (UID 65532)
# 5. Only contains Helm binary and minimal user files
# 6. No shell, no libraries, no package manager - zero attack surface
# 7. Specific version pinning to avoid unexpected updates
# 8. Statically linked binary required for scratch images

# Build with no context
# cat Containerfile | podman build -t helm-secure:latest -

# For completely isolated test (no network, no Kubernetes):
# podman run --rm \
#   --read-only \
#   --tmpfs /tmp:noexec,nosuid,nodev,size=10m \
#   --security-opt=no-new-privileges \
#   --cap-drop=ALL \
#   --user=65532:65532 \
#   --network=none \
#   --memory=64m \
#   --memory-swap=64m \
#   helm-secure:latest \
#   version --client
#
# For Kubernetes cluster access:
# podman run --rm -it \
#   --read-only \
#   --tmpfs /tmp:noexec,nosuid,nodev,size=50m \
#   --security-opt=no-new-privileges \
#   --cap-drop=ALL \
#   --user=65532:65532 \
#   --memory=256m \
#   --memory-swap=256m \
#   --cpus=1.0 \
#   --net=host \
#   -v ~/.kube/config:/tmp/kubeconfig:ro \
#   -v $(pwd)/charts:/charts:ro \
#   -e KUBECONFIG=/tmp/kubeconfig \
#   helm-secure:latest \
#   list --all-namespaces
