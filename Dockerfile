FROM registry.k8s.io/build-image/debian-base:bookworm-v1.0.0

# https://docs.docker.com/reference/dockerfile/#automatic-platform-args-in-the-global-scope
ARG TARGETOS
ARG TARGETARCH

COPY ./bin/csi-driver-rclone_${TARGETOS}_${TARGETARCH} /csi-driver-rclone

RUN set -ex && \
	apt update && \
	apt upgrade -y && \
	apt-mark unhold libcap2 && \
	clean-install bash ca-certificates curl fuse3 mount netbase procps psutils

ENTRYPOINT ["/csi-driver-rclone"]

# Labels, for details see http://label-schema.org/rc1/
ARG BUILD_DATE
ARG BUILD_VERSION
ARG BUILD_COMMIT

LABEL org.opencontainers.image.created  ${BUILD_DATE}
LABEL org.opencontainers.image.authors  cornfeedhobo
LABEL org.opencontainers.image.url      https://github.com/cornfeedhobo/csi-driver-rclone
LABEL org.opencontainers.image.source   https://github.com/cornfeedhobo/csi-driver-rclone
LABEL org.opencontainers.image.version  ${BUILD_VERSION}
LABEL org.opencontainers.image.revision ${BUILD_COMMIT}
LABEL org.opencontainers.image.licenses MIT
