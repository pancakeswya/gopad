package livecode

import (
	"github.com/pancakeswya/ot"
	"encoding/json"
	"errors"
)

type UserOperation struct {
	Id       uint64       `json:"id"`
	Sequence *ot.Sequence `json:"operation"`
}

type ClientInfo struct {
	Name string `json:"name"`
	Hue  uint32 `json:"hue"`
}

type CursorData struct {
	Cursors    []uint32    `json:"cursors"`
	Selections [][2]uint32 `json:"selections"`
}

type Edit struct {
	Revision  int          `json:"revision"`
	Operation *ot.Sequence `json:"operation"`
}

type SetLanguage string

type ClientMsg interface {
	IsClientMsg()
}

func (Edit) IsClientMsg()        {}
func (SetLanguage) IsClientMsg() {}
func (ClientInfo) IsClientMsg()  {}
func (CursorData) IsClientMsg()  {}

type History struct {
	Start      int             `json:"start"`
	Operations []UserOperation `json:"operations"`
}

type UserInfo struct {
	Id   uint64      `json:"id"`
	Info *ClientInfo `json:"info"`
}

type UserCursor struct {
	Id   uint64     `json:"id"`
	Data CursorData `json:"data"`
}

type ServerMsg interface {
	IsServerMsg()
}

type IdentityServerMsg struct {
	Identity uint64 `json:"Identity"`
}

type HistoryServerMsg struct {
	History History `json:"History"`
}

type LanguageServerMsg struct {
	Language string `json:"Language"`
}

type UserInfoServerMsg struct {
	UserInfo UserInfo `json:"UserInfo"`
}

type UserCursorServerMsg struct {
	UserCursor UserCursor `json:"UserCursor"`
}

func (IdentityServerMsg) IsServerMsg()   {}
func (HistoryServerMsg) IsServerMsg()    {}
func (LanguageServerMsg) IsServerMsg()   {}
func (UserInfoServerMsg) IsServerMsg()   {}
func (UserCursorServerMsg) IsServerMsg() {}

func unmarshalClientMsg(b json.RawMessage) (ClientMsg, error) {
	var clientMsg struct {
		Edit        *Edit        `json:"Edit"`
		SetLanguage *SetLanguage `json:"SetLanguage"`
		ClientInfo  *ClientInfo  `json:"ClientInfo"`
		CursorData  *CursorData  `json:"CursorData"`
	}
	if err := json.Unmarshal(b, &clientMsg); err != nil {
		return nil, err
	}
	if clientMsg.Edit != nil {
		return *clientMsg.Edit, nil
	}
	if clientMsg.SetLanguage != nil {
		return *clientMsg.SetLanguage, nil
	}
	if clientMsg.ClientInfo != nil {
		return *clientMsg.ClientInfo, nil
	}
	if clientMsg.CursorData != nil {
		return *clientMsg.CursorData, nil
	}
	return nil, errors.New("not supported json schema")
}
