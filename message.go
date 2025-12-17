package filesyncer

import (
	"bytes"
	"errors"
	"fmt"
)

type MsgType byte

const (
	MsgTypeCheck     MsgType = 'C'
	MsgTypeMatch     MsgType = 'M'
	MsgTypeData      MsgType = 'D'
	MsgTypeFinish    MsgType = 'F'
	MsgTypeUndefined MsgType = 'U'
	MsgTypeAuth      MsgType = 'A'
	MsgTypeAuthOK    MsgType = 'O'
	MsgTypeAuthFail  MsgType = 'X'
)

// Phat struct
type Message struct {
	Type     MsgType
	FileName string
	Data     []byte
	MD5      string
	Match    bool
}

func (msg *Message) AsBytesBuf() []byte {
	buf := []byte{}

	switch msg.Type {
	case MsgTypeFinish, MsgTypeAuthOK, MsgTypeAuthFail:
		buf = fmt.Appendf(buf, "%c:,", msg.Type)

	case MsgTypeAuth:
		buf = fmt.Appendf(buf, "%c:,", msg.Type)
		buf = append(buf, msg.Data...)

	case MsgTypeCheck:
		buf = fmt.Appendf(buf, "%c:%s,%s", msg.Type, msg.FileName, msg.MD5)

	case MsgTypeMatch:
		matchValue := 0
		if msg.Match {
			matchValue = 1
		}

		buf = fmt.Appendf(buf, "%c:%s,%d", msg.Type, msg.FileName, matchValue)

	case MsgTypeData:
		buf = fmt.Appendf(buf, "%c:%s,", msg.Type, msg.FileName)
		buf = append(buf, msg.Data...)

	case MsgTypeUndefined:
		// Leaving this panic here like an assert
		panic("Got undefined Msg type when trying to create msg buf. This shouldn't happen.")
	}

	// end message
	buf = append(buf, '\x00')

	return buf
}

// parse the format `<MsgType>:<r-filepath>,{...}\n`
// {...} Is then the relevant data depending on the MsgType
func ParseMessage(msgStream []byte) (Message, error) {

	msg := Message{Type: MsgTypeUndefined}

	// Smallest msg is: "F:,\x00"
	if len(msgStream) < 4 {
		return msg, errors.New("Message too short")
	}

	split := bytes.SplitAfterN(msgStream[2:], []byte(","), 2)
	msg.FileName = string(split[0][:len(split[0])-1])

	if !bytes.HasSuffix(split[1], []byte("\x00")) {
		return msg, errors.New("Msg does not end in expected null byte")
	}

	switch MsgType(msgStream[0]) {
	case MsgTypeFinish:
		msg.Type = MsgTypeFinish

	case MsgTypeAuth:
		msg.Type = MsgTypeAuth
		msg.Data = append(msg.Data, split[1][:len(split[1])-1]...)

	case MsgTypeAuthOK:
		msg.Type = MsgTypeAuthOK

	case MsgTypeAuthFail:
		msg.Type = MsgTypeAuthFail

	case MsgTypeCheck:
		msg.Type = MsgTypeCheck
		msg.MD5 = string(split[1][:len(split[1])-1])

	case MsgTypeMatch:
		msg.Type = MsgTypeMatch
		// Only exect one value after filename in format
		switch split[1][0] {
		case '0':
			msg.Match = false
		case '1':
			msg.Match = true
		default:
			return msg, fmt.Errorf("Expected 1 or 0 on MsgCheck response. Got: %c. Full byte slice %s.", split[1][0], string(split[1]))
		}

	case MsgTypeData:
		msg.Type = MsgTypeData
		msg.Data = append(msg.Data, split[1][:len(split[1])-1]...)

	default:
		return msg, errors.New("Could not parse error bad starting value in msg")
	}
	return msg, nil
}
