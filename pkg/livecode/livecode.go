package livecode

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/gorilla/websocket"
	"github.com/pancakeswya/ot"
	"math"
	"github.com/teivah/broadcast"
	"context"
	"encoding/json"
	"slices"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Notify struct{}

type Livecode struct {
	stateMtx sync.RWMutex
	state    State

	count  atomic.Uint64
	notify *broadcast.Relay[Notify]
	update *broadcast.Relay[ServerMsg]
	killed atomic.Bool

	logger zerolog.Logger
}

type State struct {
	operations []UserOperation
	text       string
	language   *string
	users      map[uint64]ClientInfo
	cursors    map[uint64]CursorData
}

func NewLivecode() *Livecode {
	return &Livecode{
		notify: broadcast.NewRelay[Notify](),
		update: broadcast.NewRelay[ServerMsg](),
		state: State{
			users:   make(map[uint64]ClientInfo),
			cursors: make(map[uint64]CursorData),
		},
		logger: log.With().Str("component", "livecode.Livecode").Logger(),
	}
}

func FromDocument(doc *Document) *Livecode {
	operation := ot.NewSequence()
	operation.Insert(doc.Text)

	livecode := NewLivecode()
	livecode.state.text = doc.Text
	livecode.state.language = doc.Language
	livecode.state.operations = append(livecode.state.operations, UserOperation{
		Id:       math.MaxUint64,
		Sequence: operation,
	})
	return livecode
}

func (livecode *Livecode) OnConnection(conn *websocket.Conn) {
	id := livecode.count.Add(1)

	if err := livecode.handleConnection(id, conn); err != nil {
		livecode.logger.Warn().Msgf("connection terminated early: %v", err)
	}
	livecode.logger.Info().Msgf("disconnection, id = %v", id)

	livecode.stateMtx.Lock()
	defer livecode.stateMtx.Unlock()
	delete(livecode.state.users, id)
	delete(livecode.state.cursors, id)
	livecode.update.Broadcast(UserInfoServerMsg{
		UserInfo: UserInfo{Id: id},
	})
}

func (livecode *Livecode) Text() string {
	livecode.stateMtx.RLock()
	defer livecode.stateMtx.RUnlock()
	return livecode.state.text
}

func (livecode *Livecode) Snapshot() *Document {
	livecode.stateMtx.RLock()
	defer livecode.stateMtx.RUnlock()
	return &Document{
		Text:     livecode.state.text,
		Language: livecode.state.language,
	}
}

func (livecode *Livecode) Revision() int {
	livecode.stateMtx.RLock()
	defer livecode.stateMtx.RUnlock()
	return len(livecode.state.operations)
}

func (livecode *Livecode) Kill() {
	livecode.killed.Store(true)
	livecode.notify.Notify(Notify{})
}

func (livecode *Livecode) Killed() bool {
	return livecode.killed.Load()
}

func (livecode *Livecode) setLanguage(language *string) {
	livecode.stateMtx.Lock()
	defer livecode.stateMtx.Unlock()
	livecode.state.language = language
}

func (livecode *Livecode) addUser(id uint64, user ClientInfo) {
	livecode.stateMtx.Lock()
	defer livecode.stateMtx.Unlock()
	livecode.state.users[id] = user
}

func (livecode *Livecode) addUserCursor(id uint64, cursor CursorData) {
	livecode.stateMtx.Lock()
	defer livecode.stateMtx.Unlock()
	livecode.state.cursors[id] = cursor
}

func (livecode *Livecode) readMessagesFromConn(conn *websocket.Conn, ctx context.Context, wg *sync.WaitGroup, resChan chan<- error, msgChan chan<- ClientMsg) {
	var err error
	wg.Add(1)
	defer func() {
		resChan <- err
		wg.Done()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			var raw json.RawMessage
			if err = conn.ReadJSON(&raw); err != nil {
				livecode.logger.Error().Err(err).Msg("failed to read json from conn")
				return
			}
			var msg ClientMsg
			msg, err = unmarshalClientMsg(raw)
			if err != nil {
				livecode.logger.Error().Err(err).Msg("failed to unmarshal client msg")
				return
			}
			msgChan <- msg
		}
	}
}

func (livecode *Livecode) handleMessage(id uint64, msg ClientMsg) error {
	switch val := msg.(type) {
	case Edit:
		if err := livecode.applyEdit(id, val.Revision, val.Operation); err != nil {
			livecode.logger.Error().Err(err).Msg("failed to apply edit")
			return err
		}
		livecode.notify.Notify(Notify{})
	case SetLanguage:
		language := string(val)
		livecode.setLanguage(&language)
		livecode.update.Broadcast(LanguageServerMsg{
			Language: language,
		})
	case ClientInfo:
		livecode.addUser(id, val)
		livecode.update.Broadcast(UserInfoServerMsg{
			UserInfo: UserInfo{
				Id:   id,
				Info: &val,
			},
		})
	case CursorData:
		livecode.addUserCursor(id, val)
		livecode.update.Broadcast(UserCursorServerMsg{
			UserCursor: UserCursor{
				Id:   id,
				Data: val,
			},
		})
	}
	return nil
}

func (livecode *Livecode) handleMessages(id uint64, revision int, updateRx *broadcast.Listener[ServerMsg], msgChan <-chan ClientMsg, resChan <-chan error, conn *websocket.Conn) (bool, error) {
	notify := livecode.notify.Listener(1)
	defer notify.Close()

	if livecode.Killed() {
		livecode.logger.Info().Msg("livecode is killed, exiting")
		return true, nil
	}
	var err error
	if livecode.Revision() > revision {
		revision, err = livecode.sendHistory(revision, conn)
		if err != nil {
			livecode.logger.Error().Err(err).Msg("failed to send history")
			return true, err
		}
	}
	select {
	case <-notify.Ch():
	case update := <-updateRx.Ch():
		if err = conn.WriteJSON(update); err != nil {
			livecode.logger.Error().Err(err).Msg("failed to send update")
			return true, err
		}
	case msg := <-msgChan:
		if err = livecode.handleMessage(id, msg); err != nil {
			livecode.logger.Error().Err(err).Msg("failed to handle message")
			return true, err
		}
	case err = <-resChan:
		livecode.logger.Error().Err(err).Msg("error in receive messages thread")
		return true, err
	}
	return false, nil
}

func (livecode *Livecode) handleConnection(id uint64, conn *websocket.Conn) error {
	updateRx := livecode.update.Listener(1)
	defer updateRx.Close()

	revision, err := livecode.sendInitial(id, conn)
	if err != nil {
		livecode.logger.Error().Err(err).Msg("failed to send initial")
		return err
	}
	resChan := make(chan error)
	msgChan := make(chan ClientMsg)

	var wg sync.WaitGroup
	ctx, cncl := context.WithCancel(context.Background())

	defer func() {
		cncl()
		wg.Wait()
	}()

	go livecode.readMessagesFromConn(conn, ctx, &wg, resChan, msgChan)

	for needExit := false; ; {
		needExit, err = livecode.handleMessages(id, revision, updateRx, msgChan, resChan, conn)
		if err != nil {
			livecode.logger.Error().Err(err).Msg("failed to handle messages, exiting")
			return err
		}
		if needExit {
			break
		}
	}
	return nil
}

func (livecode *Livecode) sendInitial(id uint64, conn *websocket.Conn) (int, error) {
	if err := conn.WriteJSON(IdentityServerMsg{Identity: id}); err != nil {
		livecode.logger.Error().Err(err).Msg("failed to send identity server msg")
		return 0, err
	}
	var messages []ServerMsg

	livecode.stateMtx.RLock()
	defer livecode.stateMtx.RUnlock()

	if len(livecode.state.operations) != 0 {
		messages = append(messages, HistoryServerMsg{
			History: History{
				Start:      0,
				Operations: slices.Clone(livecode.state.operations),
			},
		})
	}
	if livecode.state.language != nil {
		messages = append(messages, LanguageServerMsg{
			Language: *livecode.state.language,
		})
	}
	for uid, info := range livecode.state.users {
		messages = append(messages, UserInfoServerMsg{
			UserInfo: UserInfo{
				Id:   uid,
				Info: &info,
			},
		})
	}
	for uid, data := range livecode.state.cursors {
		messages = append(messages, UserCursorServerMsg{
			UserCursor: UserCursor{
				Id:   uid,
				Data: data,
			},
		})
	}
	for _, message := range messages {
		if err := conn.WriteJSON(message); err != nil {
			livecode.logger.Error().Err(err).Msg("failed to write messages at sendInitial")
			return 0, err
		}
	}
	return len(livecode.state.operations), nil
}

func (livecode *Livecode) applySequence(revision int, sequence *ot.Sequence) (string, error) {
	livecode.stateMtx.RLock()
	defer livecode.stateMtx.RUnlock()

	if opsLen := len(livecode.state.operations); revision > opsLen {
		livecode.logger.Error().Msg("invalid revision")
		return "", fmt.Errorf("got revision %d, but current is %d", revision, opsLen)
	}

	for _, operation := range livecode.state.operations[revision:] {
		newSequence, _, err := sequence.Transform(operation.Sequence)
		if err != nil {
			livecode.logger.Error().Err(err).Msg("failed to transform operation")
			return "", err
		}
		sequence = newSequence
	}
	if sequence.TargetLen > 256*1024 {
		livecode.logger.Error().Msg("sequence target too large")
		return "", fmt.Errorf("target length %d is greater than 100 KB maximum", sequence.TargetLen)
	}
	apply, err := sequence.Apply(livecode.state.text)
	if err != nil {
		livecode.logger.Error().Err(err).Msg("failed to apply sequence")
		return "", err
	}
	return apply, nil
}

func (livecode *Livecode) applyEdit(id uint64, revision int, operation *ot.Sequence) error {
	newText, err := livecode.applySequence(revision, operation)
	if err != nil {
		livecode.logger.Error().Err(err).Msg("failed to apply sequence edit")
		return err
	}
	livecode.stateMtx.Lock()
	defer livecode.stateMtx.Unlock()

	for _, data := range livecode.state.cursors {
		for i, cursor := range data.Cursors {
			data.Cursors[i] = operation.TransformIndex(cursor)
		}
		for i, selection := range data.Selections {
			data.Selections[i][0] = operation.TransformIndex(selection[0])
			data.Selections[i][1] = operation.TransformIndex(selection[1])
		}
	}
	livecode.state.operations = append(livecode.state.operations, UserOperation{
		Id:       id,
		Sequence: operation,
	})
	livecode.state.text = newText

	return nil
}

func (livecode *Livecode) getOperations(start int) ([]UserOperation, int) {
	livecode.stateMtx.RLock()
	defer livecode.stateMtx.RUnlock()

	var operations []UserOperation
	if start < len(livecode.state.operations) {
		operations = livecode.state.operations[start:]
	}
	return operations, len(operations)
}

func (livecode *Livecode) sendHistory(start int, conn *websocket.Conn) (int, error) {
	operations, operationsLen := livecode.getOperations(start)
	if operationsLen > 0 {
		message := HistoryServerMsg{
			History: History{
				Start:      start,
				Operations: operations,
			},
		}
		if err := conn.WriteJSON(message); err != nil {
			livecode.logger.Error().Err(err).Msg("failed to write json message at sendHistory")
			return 0, err
		}
	}
	return start + operationsLen, nil
}
