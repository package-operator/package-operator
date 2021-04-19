FROM scratch

WORKDIR /
COPY passwd /etc/passwd
COPY addon-operator-manager /

USER "noroot"

ENTRYPOINT ["/addon-operator-manager"]
