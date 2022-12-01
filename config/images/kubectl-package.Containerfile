FROM scratch

WORKDIR /
COPY passwd /etc/passwd
COPY kubectl-package /

USER "noroot"

ENTRYPOINT ["/kubectl-package"]
