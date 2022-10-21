FROM alpine

WORKDIR /
COPY package-operator-manager /

RUN apk add ca-certificates --no-cache
USER noroot
ENTRYPOINT ["/package-operator-manager"]
