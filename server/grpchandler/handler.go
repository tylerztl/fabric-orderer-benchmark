package grpchandler

import "fabric-orderer-benchmark/server/ote"

type Handler struct {
	engine *ote.OrdererTrafficEngine
}

var hanlder = newHandler()

func init() {
	hanlder.engine = ote.NewOTE()

	go hanlder.engine.StartConsumer("localhost:7050", "mychannel1", 0)
}

func newHandler() *Handler {
	return &Handler{}
}

func getEngine() *ote.OrdererTrafficEngine {
	return hanlder.engine
}
