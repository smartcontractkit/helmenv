ARG TAG=1.9.14
FROM solanalabs/solana:v$TAG

EXPOSE 8899/tcp
EXPOSE 8900/tcp
EXPOSE 1027/udp
EXPOSE 1024/udp

RUN apt-get install -y bash

ENTRYPOINT ["solana-test-validator", "-r", "--mint", "AAxAoGfkbWnbgsiQeAanwUvjv6bQrM5JS8Vxv1ckzVxg"]