ARG REGISTRY=docker.io
ARG ALPINE_VER=3
ARG GO_VER=1.16-alpine

FROM ${REGISTRY}/library/golang:${GO_VER} as golang
RUN apk add --no-cache \
      ca-certificates \
      git \
      make
RUN adduser -D appuser
WORKDIR /src

FROM golang as dev
COPY . /src/
ENV PATH=${PATH}:/src/bin
CMD make bin/reproducible-proxy && bin/reproducible-proxy

FROM dev as build
RUN make bin/reproducible-proxy
USER appuser
CMD [ "bin/reproducible-proxy" ]

FROM ${REGISTRY}/library/alpine:${ALPINE_VER} as release-alpine
COPY --from=build /etc/passwd /etc/group /etc/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build --chown=appuser /home/appuser /home/appuser
COPY --from=build /src/bin/reproducible-proxy /usr/local/bin/reproducible-proxy
USER appuser
CMD [ "reproducible-proxy", "--help" ]

ARG BUILD_DATE
ARG VCS_REF
LABEL maintainer="" \
      org.opencontainers.image.created=$BUILD_DATE \
      org.opencontainers.image.authors="sudo-bmitch" \
      org.opencontainers.image.url="https://github.com/sudo-bmitch/reproducible-proxy" \
      org.opencontainers.image.documentation="https://github.com/sudo-bmitch/reproducible-proxy" \
      org.opencontainers.image.source="https://github.com/sudo-bmitch/reproducible-proxy" \
      org.opencontainers.image.version="latest" \
      org.opencontainers.image.revision=$VCS_REF \
      org.opencontainers.image.vendor="" \
      org.opencontainers.image.licenses="Apache 2.0" \
      org.opencontainers.image.title="reproducible-proxy" \
      org.opencontainers.image.description=""

FROM scratch as release-scratch
COPY --from=build /etc/passwd /etc/group /etc/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build --chown=appuser /home/appuser /home/appuser
COPY --from=build /src/bin/reproducible-proxy /reproducible-proxy
USER appuser
ENTRYPOINT [ "/reproducible-proxy" ]

ARG BUILD_DATE
ARG VCS_REF
LABEL maintainer="" \
      org.opencontainers.image.created=$BUILD_DATE \
      org.opencontainers.image.authors="sudo-bmitch" \
      org.opencontainers.image.url="https://github.com/sudo-bmitch/reproducible-proxy" \
      org.opencontainers.image.documentation="https://github.com/sudo-bmitch/reproducible-proxy" \
      org.opencontainers.image.source="https://github.com/sudo-bmitch/reproducible-proxy" \
      org.opencontainers.image.version="latest" \
      org.opencontainers.image.revision=$VCS_REF \
      org.opencontainers.image.vendor="" \
      org.opencontainers.image.licenses="Apache 2.0" \
      org.opencontainers.image.title="reproducible-proxy" \
      org.opencontainers.image.description=""
