FROM registry.ci.openshift.org/openshift/release:golang-1.16

RUN yum install -y \
  docker \
  python3-pip \
  sudo \
  pip3 install pre-commit

WORKDIR /workdir

COPY . .
