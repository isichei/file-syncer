package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMsgParserMatch(t *testing.T) {
	expectedMsg := Message{Type: MsgTypeMatch, FileName:"bob.md", Match: true}
	actualMsg, err := ParseMessage([]byte("M:bob.md,1\x00"))
	assert.Equal(t, err, nil, "Should not error")
	assert.Equal(t, actualMsg, expectedMsg, "they should be equal")
}


func TestMsgParserCheck(t *testing.T) {
	expectedMsg := Message{Type: MsgTypeCheck, FileName:"bob.md", MD5: "test"}
	actualMsg, err := ParseMessage([]byte("C:bob.md,test\x00"))
	assert.Equal(t, err, nil, "Should not error")
	assert.Equal(t, actualMsg, expectedMsg, "they should be equal")
}

func TestMsgParserData(t *testing.T) {
	expectedMsg := Message{Type: MsgTypeData, FileName:"bob.md", Data: []byte("#Title\n\n#Description\n\nSome text.\n")}
	actualMsg, err := ParseMessage([]byte("D:bob.md,#Title\n\n#Description\n\nSome text.\n\x00"))
	assert.Equal(t, err, nil, "Should not error")
	assert.Equal(t, actualMsg, expectedMsg, "they should be equal")
}
