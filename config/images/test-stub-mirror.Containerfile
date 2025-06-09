FROM scratch

WORKDIR /
COPY ./passwd /etc/passwd
COPY ./test-stub-mirror /test-stub-mirror

USER 10001

ENTRYPOINT ["/test-stub-mirror"]
