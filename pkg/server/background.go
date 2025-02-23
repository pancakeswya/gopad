package server

import (
	"time"
	"github.com/rs/zerolog/log"
	"github.com/pancakeswya/gopad/pkg/livecode"
)

const (
	persistInterval = 3 * time.Second
	cleanInterval   = time.Hour
)

func (state *State) RunCleaner(expiryDays int) {
	go state.runIntervaled(cleanInterval, func() bool {
		var keysToDelete []string
		state.documents.Range(func(key, value interface{}) bool {
			doc := value.(*Document)
			doc.mtx.RLock()
			defer doc.mtx.RUnlock()

			if time.Since(doc.lastAccessed) > time.Duration(expiryDays)*24*time.Hour {
				keysToDelete = append(keysToDelete, key.(string))
			}
			return true
		})
		log.Info().Msgf("cleaner removing keys: %v", keysToDelete)
		for _, key := range keysToDelete {
			state.documents.Delete(key)
		}
		return true
	})
}

func (state *State) runPersister(id string, livecode *livecode.Livecode) {
	lastRevision := 0
	go state.runIntervaled(persistInterval, func() bool {
		revision := livecode.Revision()
		if revision > lastRevision {
			log.Info().Msgf("persisting revision %d for id = %s", revision, id)
			if err := state.database.Store(id, livecode.Snapshot()); err != nil {
				log.Error().Msgf("error when persisting document %s: %v", id, err)
			} else {
				lastRevision = revision
			}
		}
		return !livecode.Killed()
	})
}

func (state *State) runIntervaled(duration time.Duration, job func() bool) {
	state.wg.Add(1)
	defer state.wg.Done()

	ticker := time.NewTicker(duration)
loop:
	for {
		select {
		case <-state.ctx.Done():
			break loop
		case <-ticker.C:
			if !job() {
				break loop
			}
		}
	}
}
