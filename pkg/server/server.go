package server

import (
	"sync"
	"context"
	"time"
	"github.com/pancakeswya/gopad/pkg/livecode"
	"github.com/pancakeswya/gopad/pkg/database"
	"github.com/pancakeswya/gopad/pkg/database/sqlite"
	"os"
	"path"
)

const (
	dbName       = "db.sql"
	defaultDbDir = "db_data"
)

type Config struct {
	ExpiryDays int
	Database   database.Database
}

func NewDefaultConfig() *Config {
	dbPath := os.Getenv("GOPAD_DB_PATH")
	if dbPath == "" {
		if err := os.MkdirAll(defaultDbDir, 0750); err != nil {
			panic(err)
		}
		dbPath = defaultDbDir
	}
	dbPath = path.Join(dbPath, dbName)
	db, err := sqlite.NewDatabase(dbPath)
	if err != nil {
		panic(err)
	}
	return &Config{
		ExpiryDays: 1,
		Database:   db,
	}
}

func (config *Config) Cleanup() {
	config.Database.Close()
}

type Document struct {
	lastAccessed time.Time
	livecode     *livecode.Livecode
	mtx          sync.RWMutex
}

func newDocument(livecode *livecode.Livecode) *Document {
	return &Document{
		lastAccessed: time.Now(),
		livecode:     livecode,
	}
}

type State struct {
	documents sync.Map
	database  database.Database

	ctx  context.Context
	cncl context.CancelFunc
	wg   sync.WaitGroup
}

func NewState(database database.Database) *State {
	ctx, cncl := context.WithCancel(context.Background())
	return &State{
		ctx:      ctx,
		cncl:     cncl,
		database: database,
	}
}

func (state *State) Close() {
	state.cncl()
	state.wg.Wait()
}

type Stats struct {
	StartTime    int64 `json:"start_time"`
	NumDocuments int   `json:"num_documents"`
	DatabaseSize int   `json:"database_size"`
}
