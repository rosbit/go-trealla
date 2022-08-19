package trealla

import (
	"sync"
	"os"
	"time"
)

type tplCtx struct {
	tpl *Trealla
	mt   time.Time
}

var (
	tplExePath string
	tplCtxCache map[string]*tplCtx
	lock *sync.Mutex
)

func InitTplCache(treallaExePath string) {
	if lock != nil {
		return
	}
	lock = &sync.Mutex{}
	tplExePath = treallaExePath
	tplCtxCache = make(map[string]*tplCtx)
}

func LoadFileFromCache(path string) (ctx *Trealla, err error) {
	lock.Lock()
	defer lock.Unlock()

	tplC, ok := tplCtxCache[path]

	if !ok {
		if ctx, err = NewTrealla(tplExePath); err != nil {
			return
		}
		if err = ctx.LoadFile(path); err != nil {
			ctx.Quit()
			ctx = nil
			return
		}
		fi, _ := os.Stat(path)
		tplC = &tplCtx{
			tpl: ctx,
			mt: fi.ModTime(),
		}
		tplCtxCache[path] = tplC
		return
	}

	fi, e := os.Stat(path)
	if e != nil {
		err = e
		return
	}
	mt := fi.ModTime()
	if tplC.mt.Before(mt) {
		if err = tplC.tpl.LoadFile(path); err != nil {
			return
		}
		tplC.mt = mt
	}
	ctx = tplC.tpl
	return
}
