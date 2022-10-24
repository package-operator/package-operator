FROM scratch

WORKDIR /
COPY passwd /etc/passwd
COPY remote-phase-manager /

USER "noroot"

ENTRYPOINT ["/remote-phase-manager"]
