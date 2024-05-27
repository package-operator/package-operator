## phase 1: CA certificates source
FROM registry.access.redhat.com/ubi9-minimal:latest AS cert-source
# this eliminates symlinks for later COPY
RUN cp -rL /etc/pki/ca-trust/extracted/pem/ /tmp

## phase 2: actual image from scratch
FROM scratch

WORKDIR /

COPY --from=cert-source /tmp/pem/ /etc/pki/ca-trust/extracted/pem/
COPY ./passwd /etc/passwd
COPY ./package-operator-manager /package-operator-manager

USER 10001

ENTRYPOINT ["/package-operator-manager"]
