FROM quay.io/podman/stable

RUN yum install -y \
  golang \
  python3-pip && \
  pip3 install pre-commit

WORKDIR /workdir

COPY . .
