FROM registry.access.redhat.com/ubi8/ubi-minimal@sha256:574f201d7ed185a9932c91cef5d397f5298dff9df08bc2ebb266c6d1e6284cd1
# registry.access.redhat.com/ubi8/ubi-minimal:8.5-240

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

COPY package-operator-webhook /usr/local/bin/

USER "noroot"

ENTRYPOINT ["/usr/local/bin/package-operator-webhook"]
