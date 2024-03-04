FROM scratch

WORKDIR /
COPY ./passwd /etc/passwd
COPY ./package-operator-webhook /package-operator-webhook

USER "noroot"

ENTRYPOINT ["/package-operator-webhook"]
