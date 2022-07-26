FROM scratch

WORKDIR /
COPY passwd /etc/passwd
COPY package-operator-manager /

USER "noroot"

ENTRYPOINT ["/package-operator-manager"]
