## Orderer Traffic Engine
This Orderer Traffic Engine (OTE) tool tests the operation of a
hyperledger fabric ordering service.

### Architecture
![ote](https://github.com/BeDreamCoder/fabric-orderer-benchmark/blob/master/OTE.jpg)

### Prerequisites
- Go 1.10+ installation or later
- GOPATH environment variable is set correctly
- Govendor version 1.0.9 or later
- Protoc Plugin
- Protocol Buffers

### Getting started
#### 1. Download fabric images
```
./scripts/download_images.sh
```
#### 2. Start the fabric network
```
make networkUp
```
#### 3. Create channel
```
make cli
```
#### 4. Start the ote server
4.1 binary start 
```
make start
```
4.2 docker start
```
docker build -t hyperledger/fabric-orderer-benchmark .
make benchmark
```

#### 5. Running the test suite
```
cd test
```

### ghz 
Simple [gRPC](http://grpc.io/) benchmarking and load testing tool inspired by [hey](https://github.com/rakyll/hey/) and [grpcurl](https://github.com/fullstorydev/grpcurl).

```
cd ghz
./ghz --config=./config.json 
```
