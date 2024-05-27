FROM scratch

WORKDIR /
COPY ./passwd /etc/passwd
COPY ./kubectl-package /kubectl-package

USER 10001

ENTRYPOINT ["/kubectl-package"]
