FROM scratch

WORKDIR /
COPY passwd /etc/passwd
COPY ocm-api-mock /

USER "noroot"

ENTRYPOINT ["/ocm-api-mock"]
