FROM alpine as alpine
RUN apk add -U --no-cache ca-certificates

FROM scratch
WORKDIR /
COPY passwd /etc/passwd
COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY package-operator-manager /
USER "noroot"
ENTRYPOINT ["/package-operator-manager"]
