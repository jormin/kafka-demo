#!/bin/bash

# login harbor
docker login harbor.wcxst.com --username $HARBOR_USERNAME --password $HARBOR_PASSWORD

# gateway
cd gateway
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o gateway main.go
docker build -t harbor.wcxst.com/kafka-demo/gateway:latest .
docker push harbor.wcxst.com/kafka-demo/gateway:latest
rm -rf ./gateway

# order
cd ../order
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o order main.go
docker build -t harbor.wcxst.com/kafka-demo/order:latest .
docker push harbor.wcxst.com/kafka-demo/order:latest
rm -rf ./order

# repository
cd ../repository
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o repository main.go
docker build -t harbor.wcxst.com/kafka-demo/repository:latest .
docker push harbor.wcxst.com/kafka-demo/repository:latest
rm -rf ./repository

# statistics
cd ../statistics
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o statistics main.go
docker build -t harbor.wcxst.com/kafka-demo/statistics:latest .
docker push harbor.wcxst.com/kafka-demo/statistics:latest
rm -rf ./statistics
