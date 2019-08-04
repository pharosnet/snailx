package snailx

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
)

func New() (x SnailX) {
	services := newLocalServiceGroup()
	x = &standaloneSnailX{
		services: services,
		snailMap: make(map[string]Snail),
		runMutex: new(sync.Mutex),
		run:      true,
		serviceBus:newServiceEventLoopBus(services),
	}
	return
}

type SnailX interface {
	Stop() (err error)
	Deploy(snail Snail) (id string)
	DeployWithOptions(snail Snail, options SnailOptions) (id string)
	UnDeploy(snailId string)
	ServiceBus() (bus ServiceBus)
}

type standaloneSnailX struct {
	serviceBus ServiceBus
	services ServiceGroup
	snailMap map[string]Snail
	runMutex *sync.Mutex
	run      bool
}

func (x *standaloneSnailX) Stop() (err error) {
	x.runMutex.Lock()
	defer x.runMutex.Unlock()
	if x.run == false {
		err = errors.New("stop failed, cause it is stopped")
		return
	}
	x.run = false
	ids := make([]string, 0, len(x.snailMap))
	for id := range x.snailMap {
		ids = append(ids, id)
	}
	wg := new(sync.WaitGroup)
	for _, id := range ids {
		wg.Add(1)
		go func(id string, x *standaloneSnailX, wg *sync.WaitGroup) {
			if snail, has := x.snailMap[id]; has {
				snail.Stop()
				wg.Done()
			}
		}(id, x, wg)
	}
	wg.Wait()
	for _, id := range ids {
		delete(x.snailMap, id)
	}
	x.services.UnDeployAll()
	return
}

func (x *standaloneSnailX) Deploy(snail Snail) (id string) {
	x.runMutex.Lock()
	defer x.runMutex.Unlock()
	if x.run == false {
		panic("deploy failed, cause it is stopped")
		return
	}
	id = fmt.Sprintf("snail-%d", len(x.snailMap)+1)
	snail.SetServiceBus(newServiceEventLoopBus(x.services))
	snail.Start()
	x.snailMap[id] = snail
	return
}

func (x *standaloneSnailX) DeployWithOptions(snail Snail, options SnailOptions) (id string) {
	x.runMutex.Lock()
	defer x.runMutex.Unlock()
	if x.run == false {
		panic("deploy failed, cause it is stopped")
		return
	}
	id = fmt.Sprintf("snail-%d", len(x.snailMap)+1)
	if options.ServiceBusKind == EVENT_SERVICE_BUS {
		snail.SetServiceBus(newServiceEventLoopBus(x.services))
	} else if options.ServiceBusKind == WORKER_SERVICE_BUS {
		workers := options.WorkersNum
		if workers <= 0 {
			workers = runtime.NumCPU() * 2
		}
		snail.SetServiceBus(newServiceWorkBus(workers, x.services))
	}
	snail.Start()
	x.snailMap[id] = snail
	return
}

func (x *standaloneSnailX) UnDeploy(snailId string) {
	x.runMutex.Lock()
	defer x.runMutex.Unlock()
	if x.run == false {
		panic("deploy failed, cause it is stopped")
		return
	}
	if snail, has := x.snailMap[snailId]; has {
		snail.Stop()
		delete(x.snailMap, snailId)
	}
}

func (x *standaloneSnailX) ServiceBus() (bus ServiceBus) {
	bus = x.serviceBus
	return
}