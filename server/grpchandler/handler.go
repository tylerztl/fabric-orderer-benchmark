package grpchandler

import "fabric-orderer-benchmark/server/ote"

type Handler struct {
	engine *ote.OrdererTrafficEngine
}

var hanlder = newHandler()

func init() {
	hanlder.engine = ote.NewOTE()
}

func newHandler() *Handler {
	return &Handler{}
}

func getEngine() *ote.OrdererTrafficEngine {
	return hanlder.engine
}
