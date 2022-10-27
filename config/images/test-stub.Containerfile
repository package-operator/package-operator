FROM scratch

WORKDIR /
COPY passwd /etc/passwd
COPY test-stub /

USER "noroot"

ENTRYPOINT ["/test-stub"]
