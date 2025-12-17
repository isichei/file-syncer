package filesyncer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMsgRoundTrip(t *testing.T) {
	tests := []struct {
		name              string
		expectedMsg       Message
		expectedMsgStream []byte
	}{
		{
			name:              "MsgTypeCheck",
			expectedMsg:       Message{Type: MsgTypeCheck, FileName: "bob.md", MD5: "test"},
			expectedMsgStream: []byte("C:bob.md,test\x00"),
		},
		{
			name:              "MsgTypeMatch",
			expectedMsg:       Message{Type: MsgTypeMatch, FileName: "bob.md", Match: true},
			expectedMsgStream: []byte("M:bob.md,1\x00"),
		},
		{
			name:              "MsgTypeData",
			expectedMsg:       Message{Type: MsgTypeData, FileName: "bob.md", Data: []byte("#Title\n\n#Description\n\nSome text.\n")},
			expectedMsgStream: []byte("D:bob.md,#Title\n\n#Description\n\nSome text.\n\x00"),
		},
		{
			name:              "MsgTypeFinish",
			expectedMsg:       Message{Type: MsgTypeFinish},
			expectedMsgStream: []byte("F:,\x00"),
		},
		{
			name:              "MsgTypeAuth",
			expectedMsg:       Message{Type: MsgTypeAuth, Data: []byte("shhhhhh!")},
			expectedMsgStream: []byte("A:,shhhhhh!\x00"),
		},
		{
			name:              "MsgTypeAuthOK",
			expectedMsg:       Message{Type: MsgTypeAuthOK},
			expectedMsgStream: []byte("O:,\x00"),
		},
		{
			name:              "MsgTypeAuthFail",
			expectedMsg:       Message{Type: MsgTypeAuthFail},
			expectedMsgStream: []byte("X:,\x00"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Convert to bytes buffer
			actualMsgStream := tc.expectedMsg.AsBytesBuf()
			assert.Equal(t, tc.expectedMsgStream, actualMsgStream, "AsBytesBuf() output mismatch")

			// Convert back to Message
			actualMsg, err := ParseMessage(actualMsgStream)
			assert.NoError(t, err, "ParseMessage should not error")
			assert.Equal(t, tc.expectedMsg, actualMsg, "ParseMessage() output mismatch")
		})
	}
}
