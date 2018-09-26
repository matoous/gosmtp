package mail

import (
	"bytes"
	"fmt"
	"net/textproto"
	"strings"
)

// RawGetHeaders return raw headers
func RawGetHeaders(env *Envelope) []byte {
	return bytes.Split(env.Data.Bytes(), []byte{13, 10, 13, 10})[0]
}

// RawHaveHeader check igf header header is present in raw mail
func RawHaveHeader(env *Envelope, header string) bool {
	var bHeader []byte
	if strings.ToLower(header) == "message-id" {
		bHeader = []byte("Message-ID")
	} else {
		bHeader = []byte(textproto.CanonicalMIMEHeaderKey(header))
	}
	for _, line := range bytes.Split(RawGetHeaders(env), []byte{13, 10}) {
		if bytes.HasPrefix(line, bHeader) {
			return true
		}
	}
	return false
}

// RawGetMessageId return Message-ID or empty string if to found
func RawGetMessageId(env *Envelope) []byte {
	fmt.Printf("%s\n", env.Data.String())
	bHeader := []byte("message-id")
	for _, line := range bytes.Split(RawGetHeaders(env), []byte{13, 10}) {
		if bytes.HasPrefix(bytes.ToLower(line), bHeader) {
			// strip <>
			return bytes.TrimPrefix(bytes.TrimSuffix(line[12:], []byte{62}), []byte{60})
		}
	}
	return []byte{}
}
