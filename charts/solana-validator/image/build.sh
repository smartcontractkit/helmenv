#!/bin/bash

TAG=1.9.14

docker build --build-arg TAG=${TAG}  -t tateexon/solana-validator:${TAG} .

# example docker push, to be updated when we have a place in ecr
# docker push tateexon/solana-validator:${TAG}