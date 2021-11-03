FROM golang:1.16

RUN apt-get update && apt-get install -y \
  docker.io \
  python3-pip \
  sudo \
  && rm -rf /var/lib/apt/lists/* && \
  pip3 install pre-commit
