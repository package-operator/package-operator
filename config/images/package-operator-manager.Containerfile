FROM alpine

WORKDIR /
COPY package-operator-manager /

RUN apk add ca-certificates --no-cache
ENTRYPOINT ["/package-operator-manager"]
