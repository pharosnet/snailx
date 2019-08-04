package snailx

const (
	EVENT_SERVICE_BUS  = "EVENT_SERVICE_BUS"
	WORKER_SERVICE_BUS = "WORKER_SERVICE_BUS"
)

type SnailOptions struct {
	ServiceBusKind string
	WorkersNum     int
}

type Snail interface {
	SetServiceBus(bus ServiceBus)
	Start()
	Stop()
}
