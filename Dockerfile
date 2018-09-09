FROM golang:1.11-stretch

RUN apt-get update \
 && apt-get install -y upx \
 && rm -rf /var/lib/apt/lists/*

WORKDIR /src