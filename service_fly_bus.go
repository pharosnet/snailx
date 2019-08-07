package snailx

import (
	"context"
	"errors"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type entry struct {
	value *serviceEvent
}

func newArray(capacity int64) (a *array) {
	if capacity > 0 && (capacity&(capacity-1)) != 0 {
		panic("The array capacity must be a power of two, e.g. 2, 4, 8, 16, 32, 64, etc.")
		return
	}
	var items []*entry = nil
	align := int64(unsafe.Alignof(items))
	mask := int64(capacity - 1)
	shift := int64(math.Log2(float64(capacity)))
	size := int64(capacity) * align
	items = make([]*entry, size)
	itemBasePtr := uintptr(unsafe.Pointer(&items[0]))
	itemMSize := unsafe.Sizeof(items[0])
	for i := int64(0); i < size; i++ {
		items[i&mask*align] = &entry{}
	}
	return &array{
		capacity:    capacity,
		size:        size,
		shift:       shift,
		align:       align,
		mask:        mask,
		items:       items,
		itemBasePtr: itemBasePtr,
		itemMSize:   itemMSize,
	}
}

// shared array
type array struct {
	capacity    int64
	size        int64
	shift       int64
	align       int64
	mask        int64
	items       []*entry
	itemBasePtr uintptr
	itemMSize   uintptr
}

func (a *array) elementAt(seq int64) (e *entry) {
	mask := a.mask
	align := a.align
	basePtr := a.itemBasePtr
	mSize := a.itemMSize
	entryPtr := basePtr + uintptr(seq&mask*align)*mSize
	e = *((*(*entry))(unsafe.Pointer(entryPtr)))
	return
}

func (a *array) set(seq int64, v *serviceEvent) {
	a.elementAt(seq).value = v
}

func (a *array) get(seq int64) (v *serviceEvent) {
	v = a.elementAt(seq).value
	return
}

const (
	padding = 7
)

// Sequence New Function, value starts from -1.
func newSequence() (seq *sequence) {
	seq = &sequence{value: int64(-1)}
	return
}

// sequence, atomic operators.
type sequence struct {
	value int64
	rhs   [padding]int64
}

// Atomic increment
func (s *sequence) Incr() (value int64) {
	times := 10
	for {
		times--
		nextValue := s.Get() + 1
		ok := atomic.CompareAndSwapInt64(&s.value, s.value, nextValue)
		if ok {
			value = nextValue
			break
		}
		time.Sleep(1 * time.Nanosecond)
		if times <= 0 {
			times = 10
			runtime.Gosched()
		}
	}
	return
}

// Atomic decrement
func (s *sequence) Decr() (value int64) {
	times := 10
	for {
		times--
		preValue := s.Get() - 1
		ok := atomic.CompareAndSwapInt64(&s.value, s.value, preValue)
		if ok {
			value = preValue
			break
		}
		time.Sleep(1 * time.Nanosecond)
		if times <= 0 {
			times = 10
			runtime.Gosched()
		}
	}
	return
}

// Atomic get Sequence value.
func (s *sequence) Get() (value int64) {
	value = atomic.LoadInt64(&s.value)
	return
}

var ErrBufSendClosed error = errors.New("can not send item into the closed buffer")
var ErrBufCloseClosed error = errors.New("can not close buffer, buffer is closed")
var ErrBufSyncUnclosed error = errors.New("can not sync buffer, buffer is not closed")

const (
	statusRunning = int64(1)
	statusClosed  = int64(0)
	ns1           = 1 * time.Nanosecond
	ms500         = 500 * time.Microsecond
)

// status: running, closed
type status struct {
	v   int64
	rhs [padding]int64
}

func (s *status) setRunning() {
	atomic.StoreInt64(&s.v, statusRunning)
}

func (s *status) isRunning() bool {
	return statusRunning == atomic.LoadInt64(&s.v)
}

func (s *status) setClosed() {
	atomic.StoreInt64(&s.v, statusClosed)
}

func (s *status) isClosed() bool {
	return statusClosed == atomic.LoadInt64(&s.v)
}

// buffer interface
type buffer interface {
	// Send item into buffer.
	Send(event *serviceEvent) (err error)
	// Recv value from buffer, if active eq false, then the buffer is closed and no remains.
	Recv() (event *serviceEvent, active bool)
	// Get remains length
	Len() (length int64)
	// Get capacity length
	Cap() (length int64)
	// Close Buffer, when closed, can not send item into buffer, but can recv remains.
	Close() (err error)
	// Sync, waiting for remains to be received. Only can be called after Close().
	Sync(ctx context.Context) (err error)
}

// Note: The array capacity must be a power of two, e.g. 2, 4, 8, 16, 32, 64, etc.
func newArrayBuffer(capacity int64) buffer {
	b := &arrayBuffer{
		capacity: capacity,
		buffer:   newArray(capacity),
		wdSeq:    newSequence(),
		wpSeq:    newSequence(),
		rdSeq:    newSequence(),
		rpSeq:    newSequence(),
		sts:      &status{},
		mutex:    &sync.Mutex{},
	}
	b.sts.setRunning()
	return b
}

type arrayBuffer struct {
	capacity int64
	buffer   *array
	wpSeq    *sequence
	wdSeq    *sequence
	rpSeq    *sequence
	rdSeq    *sequence
	sts      *status
	mutex    *sync.Mutex
}

func (b *arrayBuffer) Send(event *serviceEvent) (err error) {
	if b.sts.isClosed() {
		err = ErrBufSendClosed
		return
	}
	next := b.wpSeq.Incr()
	times := 10
	for {
		times--
		if next-b.capacity-b.rdSeq.Get() <= 0 && next == b.wdSeq.Get()+1 {
			b.buffer.set(next, event)
			b.wdSeq.Incr()
			break
		}
		time.Sleep(ns1)
		if times <= 0 {
			runtime.Gosched()
			times = 10
		}
	}
	return
}

func (b *arrayBuffer) Recv() (event *serviceEvent, active bool) {
	active = true
	if b.sts.isClosed() && b.Len() == int64(0) {
		active = false
		return
	}
	times := 10
	next := b.rpSeq.Incr()
	for {

		if next <= b.wdSeq.Get() && next == b.rdSeq.Get()+1 {
			event = b.buffer.get(next)
			b.rdSeq.Incr()
			break
		}
		time.Sleep(ns1)
		if times <= 0 {
			runtime.Gosched()
			times = 10
		}
	}
	return
}

func (b *arrayBuffer) Len() (length int64) {
	length = b.wpSeq.Get() - b.rdSeq.Get()
	return
}

func (b *arrayBuffer) Cap() (length int64) {
	return b.capacity
}

func (b *arrayBuffer) Close() (err error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.sts.isClosed() {
		err = ErrBufCloseClosed
		return
	}
	b.sts.setClosed()
	return
}

func (b *arrayBuffer) Sync(ctx context.Context) (err error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.sts.isRunning() {
		err = ErrBufSyncUnclosed
		return
	}
	for {
		ok := false
		select {
		case <-ctx.Done():
			ok = true
			break
		default:
			if b.Len() <= int64(0) {
				ok = true
				break
			}
			time.Sleep(ms500)
		}
		if ok {
			break
		}
	}
	return
}

func newServiceEventFlyBus(capacity int, group ServiceGroup) ServiceBus {
	if capacity > 0 && (capacity&(capacity-1)) != 0 {
		panic("The array capacity must be a power of two, e.g. 2, 4, 8, 16, 32, 64, etc.")
		return nil
	}
	return &serviceEventFlyBus{
		capacity:  int64(capacity),
		run:      false,
		runMutex: new(sync.RWMutex),
		services: group,
	}
}

type serviceEventFlyBus struct {
	capacity int64
	boss     buffer
	workers  []buffer
	run      bool
	runMutex *sync.RWMutex
	services ServiceGroup
}

func (s *serviceEventFlyBus) start() (err error) {
	s.runMutex.Lock()
	defer s.runMutex.Unlock()
	if s.run == true {
		err = errors.New("start failed, cause it is running")
		return
	}
	s.run = true
	cpus := runtime.NumCPU()
	s.boss = newArrayBuffer(s.capacity)
	s.workers = make([]buffer, cpus)
	for i := 0; i < cap(s.workers); i++ {
		s.workers[i] = newArrayBuffer(s.capacity)
	}
	go func(s *serviceEventFlyBus) {
		workerNum := cap(s.workers)
		workerStartWg := new(sync.WaitGroup)
		for i := 0; i < workerNum; i++ {
			workerStartWg.Add(1)
			go func(workerNo int, s *serviceEventFlyBus, workerStartWg *sync.WaitGroup) {
				worker := s.workers[workerNo]
				workerStartWg.Done()
				for {
					event, ok := worker.Recv()
					if !ok {
						break
					}
					if event.address == "" {
						panic("invoke service failed, address is empty")
					}
					if event.cb == nil {
						panic(fmt.Sprintf("ServiceCallback is nil, address is %s", event.address))
					}
					s.services.Invoke(event.address, event.arg, event.cb)
					event.arg = nil
					event.address = ""
					event.cb = nil
					serviceEventPool.Put(event)
				}
			}(i, s, workerStartWg)
		}
		workerStartWg.Wait()
		workerNo := 0
		workerNumBase := workerNum - 1
		for {
			event, ok := s.boss.Recv()
			if !ok {
				break
			}
			times := 0
			for {
				times++
				worker := s.workers[workerNo & workerNumBase]
				workerNo ++
				if times >= workerNum*2 || worker.Len() < worker.Cap() {
					if err := worker.Send(event); err != nil {
						panic(fmt.Errorf("send event to worker failed, %v", err))
					}
					break
				}
			}
		}
	}(s)
	return
}

func (s *serviceEventFlyBus) stop() (err error) {
	s.runMutex.Lock()
	defer s.runMutex.Unlock()
	if s.run == false {
		err = errors.New("stop failed, cause it is stopped")
		return
	}
	s.run = false
	if cause := s.boss.Close(); cause != nil {
		err = cause
		return
	}
	if cause := s.boss.Sync(context.Background()); cause != nil {
		err = cause
		return
	}

	for _, worker := range s.workers {
		if cause := worker.Close(); cause != nil {
			err = cause
			return
		}
		if cause := worker.Sync(context.Background()); cause != nil {
			err = cause
			return
		}
	}
	return
}

func (s *serviceEventFlyBus) Deploy(address string, service Service) (err error) {
	if address == "" {
		err = fmt.Errorf("deploy service failed, address is empty")
		return
	}
	if service == nil {
		err = fmt.Errorf("deploy service failed, service is nil")
	}
	err = s.services.Deploy(address, service)
	return
}

func (s *serviceEventFlyBus) UnDeploy(address string) (err error) {
	if address == "" {
		err = fmt.Errorf("undeploy service failed, address is empty")
		return
	}
	err = s.services.UnDeploy(address)
	return
}

func (s *serviceEventFlyBus) Invoke(address string, arg interface{}, cb ServiceCallback) (err error) {
	s.runMutex.RLock()
	defer s.runMutex.RUnlock()
	if s.run == false {
		err = errors.New("invoke service failed, cause it is stopped")
		return
	}
	if address == "" {
		err = fmt.Errorf("invoke service failed, address is empty")
		return
	}
	if cb == nil {
		err = fmt.Errorf("invoke service failed, cb is nil")
		return
	}
	event, ok := serviceEventPool.Get().(*serviceEvent)
	if !ok {
		panic("snailx: get event from pool failed, bad type")
	}
	event.address = address
	event.arg = arg
	event.cb = cb
	err = s.boss.Send(event)
	return
}