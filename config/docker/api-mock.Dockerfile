FROM scratch

WORKDIR /
COPY passwd /etc/passwd
COPY api-mock /

USER "noroot"

ENTRYPOINT ["/api-mock"]
