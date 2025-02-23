package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/pancakeswya/gopad/pkg/livecode"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Consider implementing proper origin checks
	},
}

func (state *State) HandleSocket(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	doc := state.getOrCreateDocument(id)

	doc.mtx.Lock()
	doc.lastAccessed = time.Now()
	doc.mtx.Unlock()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Could not upgrade connection", http.StatusInternalServerError)
		return
	}
	doc.livecode.OnConnection(conn)
}

func (state *State) HandleText(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	text := state.getText(id)
	if n, err := w.Write([]byte(text)); err != nil || n != len(text) {
		http.Error(w, "Could not write text", http.StatusInternalServerError)
	}
}

func (state *State) HandleStats(w http.ResponseWriter, r *http.Request) {
	_ = r

	if stats, err := state.getStats(); err != nil {
		http.Error(w, "failed to get stats", http.StatusInternalServerError)
	} else if err = json.NewEncoder(w).Encode(stats); err != nil {
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
		if dbDoc, err := state.database.Load(id); err == nil {
			code = livecode.FromDocument(dbDoc)
		} else {
			code = livecode.NewLivecode()
		}
	} else {
		code = livecode.NewLivecode()
	}
	doc = newDocument(code)

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
	state.documents.Range(func(_, _ interface{}) bool {
		count++
		return true
	})

	dbSize := 0
	if state.database != nil {
		var err error
		dbSize, err = state.database.Count()
		if err != nil {
			return Stats{}, err
		}
	}
	return Stats{
		StartTime:    time.Now().Unix(),
		NumDocuments: count,
		DatabaseSize: dbSize,
	}, nil
}
