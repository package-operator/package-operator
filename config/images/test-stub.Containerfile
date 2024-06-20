FROM scratch

WORKDIR /
COPY ./passwd /etc/passwd
COPY ./test-stub /test-stub

USER 10001

ENTRYPOINT ["/test-stub"]
