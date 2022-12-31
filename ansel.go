package ansel

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"log"
	"sync"
)

type AnselConfig[I comparable] func(c *Ansel[I])
type Loader func(uri string) ([]byte, error)
type LoaderCallback func([]byte) []byte

type ResourceSetter interface {
	SetResource(resource fyne.Resource)
}

type Ansel[I comparable] struct {
	cache   map[string]fyne.Resource
	pending map[I]ResourceSetter
	cancel  map[*widget.Icon]I
	locker  sync.RWMutex

	loader         Loader
	loadedCallback LoaderCallback

	workerCount int
	workChan    chan unit[I]
}

func NewAnsel[I comparable](opts ...AnselConfig[I]) *Ansel[I] {
	a := &Ansel[I]{
		cache:       make(map[string]fyne.Resource),
		pending:     make(map[I]ResourceSetter),
		cancel:      make(map[*widget.Icon]I),
		loader:      LoadHTTP,
		workerCount: 10,
	}

	for _, opt := range opts {
		opt(a)
	}

	a.workChan = make(chan unit[I], 512)

	for w := 0; w < a.workerCount; w++ {
		go a.worker()
	}

	return a
}

func (a *Ansel[I]) Load(id I, source string, icon ResourceSetter) {
	a.workChan <- unit[I]{id: id, source: source, target: icon}
}

func (a *Ansel[I]) worker() {
	for {
		select {
		case u := <-a.workChan:
			a.doLoad(u)
		}
	}
}

func (a *Ansel[I]) doLoad(u unit[I]) {
	a.locker.RLock()
	if resource, ok := a.cache[u.source]; ok {
		u.target.SetResource(resource)
		a.locker.RUnlock()
		return
	}
	a.locker.RUnlock()

	a.locker.Lock()
	if _, ok := a.pending[u.id]; ok {
		a.pending[u.id] = u.target
		a.locker.Unlock()
		return
	}
	a.pending[u.id] = u.target
	a.locker.Unlock()

	if data, err := a.loader(u.source); err == nil {
		a.locker.Lock()
		if a.loadedCallback != nil {
			data = a.loadedCallback(data)
		}
		res := fyne.NewStaticResource(u.source, data)
		a.cache[u.source] = res
		if icn, ok := a.pending[u.id]; ok {
			icn.SetResource(res)
		}
		delete(a.pending, u.id)
		a.locker.Unlock()

	} else {
		log.Println("error loading resource", err)
	}
}

func SetLoader[I comparable](loader Loader) AnselConfig[I] {
	return func(a *Ansel[I]) {
		a.loader = loader
	}
}

func SetLoadedCallback[I comparable](loadedCallback LoaderCallback) AnselConfig[I] {
	return func(a *Ansel[I]) {
		a.loadedCallback = loadedCallback
	}
}

func SetWorkerCount[I comparable](count int) AnselConfig[I] {
	return func(a *Ansel[I]) {
		a.workerCount = count
	}
}
