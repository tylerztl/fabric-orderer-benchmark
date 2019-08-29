## Orderer Traffic Engine
This Orderer Traffic Engine (OTE) tool tests the operation of a
hyperledger fabric ordering service.

### Prerequisites
- Go 1.10+ installation or later
- GOPATH environment variable is set correctly
- Govendor version 1.0.9 or later
- Protoc Plugin
- Protocol Buffers

### Getting started
Download fabric images
```
./scripts/download_images.sh
```
Start the fabric network
```
make networkUp
```
Start the fabric-sdk-go server
```
make start
```
Running the test suite
```
cd test
```

### ghz 
Simple [gRPC](http://grpc.io/) benchmarking and load testing tool inspired by [hey](https://github.com/rakyll/hey/) and [grpcurl](https://github.com/fullstorydev/grpcurl).

```
cd ghz
./ghz --config=./config.json 
```
