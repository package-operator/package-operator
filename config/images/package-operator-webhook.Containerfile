FROM scratch

WORKDIR /
COPY ./passwd /etc/passwd
COPY ./package-operator-webhook /package-operator-webhook

USER 10001

ENTRYPOINT ["/package-operator-webhook"]
