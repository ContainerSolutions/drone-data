#!/bin/bash
set -e

docker build -t drone-test .
docker stop drone-test
docker rm drone-test
docker run -p 8000:8000 -d --name drone-test --link drone-data:drone-data drone-test
