ARG REGISTRY=docker.io
ARG ALPINE_VER=3
ARG GO_VER=1.19-alpine
ARG NODE_VER=18-alpine

FROM ${REGISTRY}/library/golang:${GO_VER} as golang
RUN apk add --no-cache \
      ca-certificates \
      make
RUN adduser -D appuser
WORKDIR /src

FROM ${REGISTRY}/library/node:${NODE_VER} as node
RUN apk add --no-cache \
      ca-certificates \
      make
RUN adduser -D appuser
WORKDIR /src

FROM node as ui
COPY ui/files/package*.json ./ui/files/
RUN  cd ui/files \
 && npm install --production
COPY . /src/
RUN make ui

FROM golang as dev
COPY . /src/
COPY --from=ui /src/ui/files/build/ /src/ui/files/build/
ENV PATH=${PATH}:/src/bin
CMD make bin/httplock && bin/httplock

FROM dev as build
RUN make bin/httplock
RUN mkdir -p /var/lib/httplock/data /var/lib/httplock/tmp \
 && chown -R appuser:appuser /var/lib/httplock
USER appuser
ENV TMPDIR=/var/lib/httplock/tmp
CMD [ "bin/httplock" ]

FROM ${REGISTRY}/library/alpine:${ALPINE_VER} as release-alpine
COPY --from=build /etc/passwd /etc/group /etc/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build --chown=appuser /home/appuser /home/appuser
COPY --from=build --chown=appuser /var/lib/httplock /var/lib/httplock
COPY --from=build /src/bin/httplock /usr/local/bin/httplock
USER appuser
CMD [ "httplock", "--help" ]

ARG BUILD_DATE
ARG VCS_REF
LABEL maintainer="" \
      org.opencontainers.image.created=$BUILD_DATE \
      org.opencontainers.image.authors="httplock maintainers" \
      org.opencontainers.image.url="https://github.com/httplock/httplock" \
      org.opencontainers.image.documentation="https://github.com/httplock/httplock" \
      org.opencontainers.image.source="https://github.com/httplock/httplock" \
      org.opencontainers.image.version="latest" \
      org.opencontainers.image.revision=$VCS_REF \
      org.opencontainers.image.vendor="" \
      org.opencontainers.image.licenses="Apache 2.0" \
      org.opencontainers.image.title="httplock" \
      org.opencontainers.image.description=""

FROM scratch as release-scratch
COPY --from=build /etc/passwd /etc/group /etc/
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build --chown=appuser /home/appuser /home/appuser
COPY --from=build --chown=appuser /var/lib/httplock /var/lib/httplock
COPY --from=build /src/bin/httplock /httplock
USER appuser
ENV TMPDIR=/var/lib/httplock/tmp
ENTRYPOINT [ "/httplock" ]

ARG BUILD_DATE
ARG VCS_REF
LABEL maintainer="" \
      org.opencontainers.image.created=$BUILD_DATE \
      org.opencontainers.image.authors="httplock maintainers" \
      org.opencontainers.image.url="https://github.com/httplock/httplock" \
      org.opencontainers.image.documentation="https://github.com/httplock/httplock" \
      org.opencontainers.image.source="https://github.com/httplock/httplock" \
      org.opencontainers.image.version="latest" \
      org.opencontainers.image.revision=$VCS_REF \
      org.opencontainers.image.vendor="" \
      org.opencontainers.image.licenses="Apache 2.0" \
      org.opencontainers.image.title="httplock" \
      org.opencontainers.image.description=""
