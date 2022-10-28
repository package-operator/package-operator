FROM quay.io/podman/stable

RUN yum install -y \
  python3-pip make ncurses git && \
  pip3 install pre-commit && \
  curl -L --fail https://go.dev/dl/go1.17.7.linux-amd64.tar.gz > /tmp/go.tar.gz && \
  rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

WORKDIR /workdir

COPY . .
