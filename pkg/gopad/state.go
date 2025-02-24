package gopad

import (
	"sync"
	"github.com/pancakeswya/gopad/pkg/database"
	"context"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"time"
	"encoding/json"
	"github.com/pancakeswya/gopad/pkg/livecode"
	"github.com/gorilla/websocket"
)

const (
	persistInterval = 3 * time.Second
	cleanInterval   = time.Hour
)

type State struct {
	documents sync.Map
	database  database.Database

	ctx      context.Context
	cncl     context.CancelFunc
	wg       sync.WaitGroup
	upgrader websocket.Upgrader

	logger zerolog.Logger
}

func NewState(database database.Database) *State {
	ctx, cncl := context.WithCancel(context.Background())
	return &State{
		ctx:      ctx,
		cncl:     cncl,
		database: database,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		logger: log.With().Str("component", "server.state").Logger(),
	}
}

func (state *State) Close() {
	state.cncl()
	state.wg.Wait()
}

func (state *State) SocketHandle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	doc := state.getOrCreateDocument(id)

	doc.mtx.Lock()
	doc.lastAccessed = time.Now()
	doc.mtx.Unlock()

	conn, err := state.upgrader.Upgrade(w, r, nil)
	if err != nil {
		state.logger.Error().Err(err).Msg("failed to upgrade connection")
		http.Error(w, "Could not upgrade connection", http.StatusInternalServerError)
		return
	}
	state.logger.Info().Msg("connection upgraded, starting livecode")
	doc.livecode.OnConnection(conn)
}

func (state *State) TextHandle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	text := state.getText(id)
	if n, err := w.Write([]byte(text)); err != nil || n != len(text) {
		state.logger.Error().Err(err).Str("path", id).Int("n", n).Msg("failed to write")
		http.Error(w, "Could not write text", http.StatusInternalServerError)
	}
}

func (state *State) StatsHandle(w http.ResponseWriter, r *http.Request) {
	_ = r

	if stats, err := state.getStats(); err != nil {
		state.logger.Error().Err(err).Msg("failed to read stats")
		http.Error(w, "failed to get stats", http.StatusInternalServerError)
	} else if err = json.NewEncoder(w).Encode(stats); err != nil {
		state.logger.Error().Err(err).Msg("failed to write stats")
		http.Error(w, "failed to encode stats", http.StatusInternalServerError)
	}
}

func (state *State) getOrCreateDocument(id string) *Document {
	var doc *Document

	docAny, loaded := state.documents.Load(id)
	if loaded {
		return docAny.(*Document)
	}
	var code *livecode.Livecode
	if state.database != nil {
		dbDoc, err := state.database.Load(id)
		if err == nil {
			code = livecode.FromDocument(dbDoc)
			goto next
		}
		state.logger.Warn().Str("id", id).Msgf("failed to load from database, err = %v", err)
	}
	code = livecode.NewLivecode()
next:
	doc = &Document{
		lastAccessed: time.Now(),
		livecode:     code,
	}

	state.documents.Store(id, doc)
	state.runPersister(id, code)

	return doc
}

func (state *State) getText(id string) string {
	if doc, ok := state.documents.Load(id); ok {
		return doc.(*Document).livecode.Text()
	}
	if state.database != nil {
		if dbDoc, err := state.database.Load(id); err == nil {
			return dbDoc.Text
		}
	}
	return ""
}

func (state *State) getStats() (Stats, error) {
	var count int
	state.documents.Range(func(_, _ any) bool {
		count++
		return true
	})
	var dbSize int
	if state.database != nil {
		var err error
		dbSize, err = state.database.Count()
		if err != nil {
			state.logger.Error().Err(err).Msg("failed to count database size")
			return Stats{}, err
		}
	}
	return Stats{
		StartTime:    time.Now().Unix(),
		NumDocuments: count,
		DatabaseSize: dbSize,
	}, nil
}

func (state *State) RunCleaner(expiryDays int) {
	go state.runIntervaled(cleanInterval, func() bool {
		var keysToDelete []string
		state.documents.Range(func(key, value any) bool {
			doc := value.(*Document)
			doc.mtx.RLock()
			defer doc.mtx.RUnlock()

			if time.Since(doc.lastAccessed) > time.Duration(expiryDays)*24*time.Hour {
				keysToDelete = append(keysToDelete, key.(string))
			}
			return true
		})
		state.logger.Info().Msgf("cleaner removing keys: %v", keysToDelete)
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
			state.logger.Info().Msgf("persisting revision %d for id = %s", revision, id)
			if err := state.database.Store(id, livecode.Snapshot()); err != nil {
				state.logger.Error().Msgf("error when persisting document %s: %v", id, err)
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
