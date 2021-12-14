FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:16da4d4c5cb289433305050a06834b7328769f8a5257ad5b4a5006465a0379ff
# registry.access.redhat.com/ubi8/ubi-minimal:8.5-204

# shadow-utils contains adduser and groupadd binaries
RUN microdnf install shadow-utils \
	&& groupadd --gid 1000 noroot \
	&& adduser \
		--no-create-home \
		--no-user-group \
		--uid 1000 \
		--gid 1000 \
		noroot

WORKDIR /

COPY addon-operator-webhook /usr/local/bin/

USER "noroot"

ENTRYPOINT ["/usr/local/bin/addon-operator-webhook"]
