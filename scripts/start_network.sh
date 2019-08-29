#!/bin/bash
#
# Copyright Ziggurat Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0
#

cd network

# Parse commandline args
while getopts "h?m:c:t:d:f:s:l:" opt; do
  case "$opt" in
    h|\?)
      printHelp
      exit 0
    ;;
    m)  MODE=$OPTARG
    ;;
  esac
done

# Print the usage message
function printHelp () {
  echo "Usage: "
  echo "      start_network.sh -m up|down|restart|cli"
  echo "      - 'up' - bring up the network with docker-compose up"
  echo "      - 'down' - clear the network with docker-compose down"
  echo "      - 'restart' - restart the network"
  echo "      - 'cli' - start the cli"
  echo "      - 'benchmark' - start benchmark"
}


function dkcl(){
        CONTAINER_IDS=$(docker ps -aq)
	echo
        if [ -z "$CONTAINER_IDS" -o "$CONTAINER_IDS" = " " ]; then
                echo "========== No containers available for deletion =========="
        else
                docker rm -f $CONTAINER_IDS
        fi
	echo
}

function dkrm(){
        DOCKER_IMAGE_IDS=$(docker images | grep "dev\|none\|test-vp\|peer[0-9]-" | awk '{print $3}')
	echo
        if [ -z "$DOCKER_IMAGE_IDS" -o "$DOCKER_IMAGE_IDS" = " " ]; then
		echo "========== No images available for deletion ==========="
        else
                docker rmi -f $DOCKER_IMAGE_IDS
        fi
	echo
}

function networkUp() {
	echo
	echo Start the network
	docker-compose up -d
	echo
}

function networkDown() {
	echo
    echo teardown the network and clean the containers and intermediate images
	docker-compose down --volumes --remove-orphans
	docker-compose -f docker-compose-cli.yaml down --volumes --remove-orphans
	docker-compose -f docker-compose-benchmark.yaml down --volumes --remove-orphans
	if [ "$MODE" != "restart" ]; then
	   dkcl
	   dkrm
	fi
}

function networkRestart() {
    networkDown
    networkUp
	echo
}

function startCli() {
	echo
	echo Start the cli
	docker-compose -f docker-compose-cli.yaml up -d
	echo
}

function startBenchmark() {
	echo
	echo Start benchmark
	docker-compose -f docker-compose-benchmark.yaml up -d
	echo
}

#Create the network using docker compose
if [ "${MODE}" == "up" ]; then
  networkUp
  elif [ "${MODE}" == "down" ]; then ## Clear the network
  networkDown
  elif [ "${MODE}" == "restart" ]; then ## Restart the network
  networkRestart
  elif [ "${MODE}" == "cli" ]; then ## start the network cli
  startCli
  elif [ "${MODE}" == "benchmark" ]; then ## start benchmark
  startBenchmark
else
  printHelp
  exit 1
fi

