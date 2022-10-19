FROM scratch

WORKDIR /
COPY passwd /etc/passwd
COPY package-loader /

USER "noroot"

ENTRYPOINT ["/package-loader"]
