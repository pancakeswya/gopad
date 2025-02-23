package database

import (
	"github.com/pancakeswya/gopad/pkg/livecode"
)

type Closable interface {
	Close()
}

type Database interface {
	Closable

	Load(id string) (*livecode.Document, error)
	Store(id string, document *livecode.Document) error
	Count() (int, error)
}
