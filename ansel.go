package ansel

import (
	"fyne.io/fyne/v2"
	lru "github.com/hashicorp/golang-lru/v2"
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
	cache   *lru.Cache[string, fyne.Resource]
	pending map[I]ResourceSetter
	cancel  map[ResourceSetter]I
	locker  sync.RWMutex

	loader         Loader
	loadedCallback LoaderCallback

	workerCount int
	workChan    chan unit[I]
}

func NewAnsel[I comparable](cacheSize int, opts ...AnselConfig[I]) *Ansel[I] {
	cache, err := lru.New[string, fyne.Resource](cacheSize)
	if err != nil {
		log.Println("error creating cache", err)
	}
	a := &Ansel[I]{
		cache:       cache,
		pending:     make(map[I]ResourceSetter),
		cancel:      make(map[ResourceSetter]I),
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
	if resource, ok := a.cache.Get(u.source); ok {
		u.target.SetResource(resource)
		a.locker.RUnlock()
		return
	}
	a.locker.RUnlock()

	a.locker.Lock()
	if _, ok := a.pending[u.id]; ok {
		a.pending[u.id] = u.target
		a.cancel[u.target] = u.id
		a.locker.Unlock()
		return
	}
	a.pending[u.id] = u.target
	a.cancel[u.target] = u.id
	a.locker.Unlock()

	if data, err := a.loader(u.source); err == nil {
		a.locker.Lock()
		if a.loadedCallback != nil {
			data = a.loadedCallback(data)
		}
		res := fyne.NewStaticResource(u.source, data)
		a.cache.Add(u.source, res)
		if id, ok := a.cancel[u.target]; ok {
			if id != u.id {
				delete(a.cancel, u.target)
			}
		} else {
			if icn, ok := a.pending[u.id]; ok {
				icn.SetResource(res)
			}
		}
		delete(a.pending, u.id)
		a.locker.Unlock()

	} else {
		// TODO: Have an error image than can be used here.
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
