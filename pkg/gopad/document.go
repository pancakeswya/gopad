package gopad

import (
	"time"
	"github.com/pancakeswya/gopad/pkg/livecode"
	"sync"
)

type Document struct {
	lastAccessed time.Time
	livecode     *livecode.Livecode
	mtx          sync.RWMutex
}
