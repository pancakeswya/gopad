package main

import (
	"github.com/pancakeswya/ot"
	"encoding/json"
)

type Sequence ot.Sequence

func NewSequence() *Sequence {
	return (*Sequence)(ot.NewSequence())
}

func FromString(s string) *Sequence {
	seq := ot.NewSequence()
	if err := json.Unmarshal([]byte(s), seq); err != nil {
		panic(err)
	}
	return (*Sequence)(seq)
}

func (seq *Sequence) Delete(n uint64) {
	(*ot.Sequence)(seq).Delete(n)
}

func (seq *Sequence) Insert(s string) {
	(*ot.Sequence)(seq).Insert(s)
}

func (seq *Sequence) Retain(n uint64) {
	(*ot.Sequence)(seq).Retain(n)
}

func (seq *Sequence) Apply(s string) string {
	r, err := (*ot.Sequence)(seq).Apply(s)
	if err != nil {
		panic(err)
	}
	return r
}

func (seq *Sequence) IsNoop() bool {
	return (*ot.Sequence)(seq).IsNoop()
}

func (seq *Sequence) Compose(other *Sequence) *Sequence {
	c, err := (*ot.Sequence)(seq).Compose((*ot.Sequence)(other))
	if err != nil {
		panic(err)
	}
	return (*Sequence)(c)
}

func (seq *Sequence) Transform(other *Sequence) (*Sequence, *Sequence) {
	a, b, err := (*ot.Sequence)(seq).Transform((*ot.Sequence)(other))
	if err != nil {
		panic(err)
	}
	return (*Sequence)(a), (*Sequence)(b)
}

func (seq *Sequence) Invert(s string) *Sequence {
	return (*Sequence)((*ot.Sequence)(seq).Invert(s))
}

func (seq *Sequence) TransformIndex(position uint32) uint32 {
	return (*ot.Sequence)(seq).TransformIndex(position)
}

func (seq *Sequence) ToString() string {
	b, err := json.Marshal((*ot.Sequence)(seq))
	if err != nil {
		panic(err)
	}
	return string(b)
}
