FROM scratch

WORKDIR /
COPY passwd /etc/passwd
COPY addon-operator-webhook /

USER "noroot"

ENTRYPOINT ["/addon-operator-webhook"]
