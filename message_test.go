package filesyncer 

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// TODO need to add tests for creating the msgStreamBuffer from a msg, should just round trip
func TestMsgMatchRoundTrip(t *testing.T) {
	expectedMsg := Message{Type: MsgTypeMatch, FileName: "bob.md", Match: true}
	expectedMsgStream := []byte("M:bob.md,1\x00")
	
	// Convert to bytes buffer
	actualMsgStream := expectedMsg.AsBytesBuf()
	assert.Equal(t, expectedMsgStream, actualMsgStream, "they should be equal")

	// Convert back to Message
	actualMsg, err := ParseMessage(actualMsgStream)
	assert.Equal(t, nil, err, "Should not error")
	assert.Equal(t, expectedMsg, actualMsg, "they should be equal")
	
	
}

func TestMsgParserCheck(t *testing.T) {
	expectedMsg := Message{Type: MsgTypeCheck, FileName: "bob.md", MD5: "test"}
	expectedMsgStream := []byte("C:bob.md,test\x00")

	// Convert to bytes buffer
	actualMsgStream := expectedMsg.AsBytesBuf()
	assert.Equal(t, expectedMsgStream, actualMsgStream, "they should be equal")

	// Convert back to Message
	actualMsg, err := ParseMessage(actualMsgStream)
	assert.Equal(t, nil, err, "Should not error")
	assert.Equal(t, expectedMsg, actualMsg, "they should be equal")
}

func TestMsgParserData(t *testing.T) {
	expectedMsg := Message{Type: MsgTypeData, FileName: "bob.md", Data: []byte("#Title\n\n#Description\n\nSome text.\n")}
	expectedMsgStream := []byte("D:bob.md,#Title\n\n#Description\n\nSome text.\n\x00")

	// Convert to bytes buffer
	actualMsgStream := expectedMsg.AsBytesBuf()
	assert.Equal(t, expectedMsgStream, actualMsgStream, "they should be equal")

	// Convert back to Message
	actualMsg, err := ParseMessage(actualMsgStream)
	assert.Equal(t, nil, err, "Should not error")
	assert.Equal(t, expectedMsg, actualMsg, "they should be equal")
}
