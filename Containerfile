# syntax=docker/dockerfile:1
ARG HELM_VERSION=v3.18.3

FROM alpine:3.22.0 AS download-verify

ARG HELM_VERSION
ARG TARGETOS
ARG TARGETARCH
ARG HELM_BASE_URL=https://get.helm.sh
ARG HELM_GITHUB_URL=https://github.com/helm/helm/releases/download/${HELM_VERSION}

RUN apk add --no-cache \
    curl \
    gnupg \
    tar \
    && rm -rf /var/cache/apk/*

RUN echo "nonroot:x:65532:65532:nonroot:/tmp:/sbin/nologin" >> /etc/passwd \
    && echo "nonroot:x:65532:" >> /etc/group

RUN curl -fsSLO "${HELM_BASE_URL}/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz" \
    && curl -fsSLO "${HELM_BASE_URL}/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz.sha256sum" \
    && curl -fsSLO "${HELM_GITHUB_URL}/helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz.asc" \
    && curl -fsSLO "${HELM_BASE_URL}/KEYS"

RUN gpg --import KEYS \
    && gpg --verify "helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz.asc" "helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz"

RUN sha256sum -c "helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz.sha256sum"

RUN tar -xzf "helm-${HELM_VERSION}-${TARGETOS}-${TARGETARCH}.tar.gz" ${TARGETOS}-${TARGETARCH}/helm \
    && mv ${TARGETOS}-${TARGETARCH}/helm /usr/local/bin/helm \
    && chmod +x /usr/local/bin/helm

RUN /usr/local/bin/helm version --client

FROM scratch

ARG HELM_VERSION

LABEL org.opencontainers.image.title="Helm"
LABEL org.opencontainers.image.description="Secure Helm container"
LABEL org.opencontainers.image.version="${HELM_VERSION#v}"
LABEL org.opencontainers.image.authors="cncf-helm-core-maintainers@lists.cncf.io"
LABEL org.opencontainers.image.documentation="https://helm.sh/docs/install"
LABEL org.opencontainers.image.source="https://github.com/helm/helm/blob/main/Containerfile"
LABEL org.opencontainers.image.licenses="Apache-2.0"

COPY --from=download-verify /usr/local/bin/helm /helm
COPY --from=download-verify /etc/passwd /etc/passwd
COPY --from=download-verify /etc/group /etc/group

USER 65532:65532
WORKDIR /workspace

ENTRYPOINT ["/helm"]
CMD ["--help"]
