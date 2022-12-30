package ansel

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"log"
	"sync"
)

type Ansel[I comparable] struct {
	cache   map[string]*fyne.StaticResource
	pending map[I]*widget.Icon
	cancel  map[*widget.Icon]I
	locker  sync.RWMutex

	loader func(uri string) ([]byte, error)
}

func NewAnsel[I comparable]() *Ansel[I] {
	return &Ansel[I]{cache: make(map[string]*fyne.StaticResource), pending: make(map[I]*widget.Icon), cancel: make(map[*widget.Icon]I), loader: LoadHTTP}
}

func (a *Ansel[I]) Load(id I, source string, icon *widget.Icon) {
	a.locker.RLock()
	if resource, ok := a.cache[source]; ok {
		icon.SetResource(resource)
		a.locker.RUnlock()
		return
	}
	a.locker.RUnlock()

	a.locker.Lock()
	if _, ok := a.pending[id]; ok {
		a.pending[id] = icon
		a.locker.Unlock()
		return
	}
	a.pending[id] = icon
	a.locker.Unlock()

	if data, err := a.loader(source); err == nil {

		a.locker.Lock()
		res := fyne.NewStaticResource(source, data)
		a.cache[source] = res
		if icn, ok := a.pending[id]; ok {
			icn.SetResource(res)
		}
		delete(a.pending, id)
		a.locker.Unlock()

	} else {
		log.Println("error loading resource", err)
	}
}
