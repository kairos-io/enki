
FROM ubuntu:latest as base
FROM busybox as builder

COPY --from=base . /rootfs

FROM rootfs
# Additional os specific things

RUN echo "nameserver 8.8.8.8" > /rootfs/etc/resolv.conf
RUN cat /rootfs/etc/resolv.conf

FROM scratch as rootfs

COPY --from=builder /rootfs/ .

FROM rootfs
# Additional os specific things

