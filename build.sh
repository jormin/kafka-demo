#!/bin/bash

# gateway
cd gateway
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o gateway main.go
docker build -t jormin/kafka-demo-gateway:v0.0.1 .
docker push jormin/kafka-demo-gateway:v0.0.1
rm -rf ./gateway

# order
cd ../order
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o order main.go
docker build -t jormin/kafka-demo-order:v0.0.1 .
docker push jormin/kafka-demo-order:v0.0.1
rm -rf ./order

# repository
cd ../repository
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o repository main.go
docker build -t jormin/kafka-demo-repository:v0.0.1 .
docker push jormin/kafka-demo-repository:v0.0.1
rm -rf ./repository

# statistics
cd ../statistics
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o statistics main.go
docker build -t jormin/kafka-demo-statistics:v0.0.1 .
docker push jormin/kafka-demo-statistics:v0.0.1
rm -rf ./statistics
