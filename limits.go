package gosmtp

import "time"

// Limits hold all the session limitations - max attempts, sizes and timeouts
type Limits struct {
	CmdInput     time.Duration // client commands
	MsgInput     time.Duration // total time for the email
	ReplyOut     time.Duration // server reply time
	TLSSetup     time.Duration // time limit for STARTTLS setup
	MsgSize      int64         // max email size
	BadCmds      int           // bad commands limit
	MaxRcptCount int           // maximum number of recipients of message
}

// DefaultLimits that are applied if you do not specify custom limits
// Two minutes for command input and command replies, ten minutes for
// receiving messages, and 5 Mbytes of message size.
//
// Note that these limits are not necessarily RFC compliant, although
// they should be enough for real email clients, TODO change this to RFC compliant
var DefaultLimits = Limits{
	CmdInput:     2 * time.Minute,
	MsgInput:     10 * time.Minute,
	ReplyOut:     2 * time.Minute,
	TLSSetup:     4 * time.Minute,
	MsgSize:      5 * 1024 * 1024,
	BadCmds:      5,
	MaxRcptCount: 200,
}
