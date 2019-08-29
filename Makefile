# Copyright zhigui Corp All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#
# -------------------------------------------------------------
# This makefile defines the following targets
#
#   - protos - generate all protobuf artifacts based on .proto files
#   - up - start the fabric network
#   - clean - teardown the fabric network and clean the containers and intermediate images
#   - cli - tart the fabric network cli
#   - satrt - start the fabric-orderer-benchmark server
#   - benchmark - start benchmark

.PHONY: protos
protos :
	./scripts/compile_protos.sh

.PHONY: up
up :
	./scripts/start_network.sh -m up

.PHONY: clean
clean :
	./scripts/start_network.sh -m down

.PHONY: cli
cli :
	./scripts/start_network.sh -m cli

.PHONY: benchmark
benchmark :
	./scripts/start_network.sh -m benchmark

.PHONY: start
start :
	go run main.go start
