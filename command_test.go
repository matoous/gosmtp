package gosmtp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommand_Args(t *testing.T) {
	cmd, err := parseCommand("MAIL")
	if err != nil {
		panic(err)
	}
	assert.Nil(t, cmd.arguments,
		"'MAIL' command args should be empty")

	cmd, err = parseCommand("MAIL arg1")
	if err != nil {
		panic(err)
	}
	assert.Equal(t, []string{"arg1"}, cmd.arguments,
		"'MAIL arg1' command arguments should contain one argument 'arg1'")

	cmd, err = parseCommand("MAIL arg1 arg2 arg3")
	if err != nil {
		panic(err)
	}
	assert.Equal(t, []string{"arg1", "arg2", "arg3"}, cmd.arguments,
		"'MAIL arg1 arg2 arg3' command arguments should contain three arguments 'arg1', 'arg2' and 'arg3'")
}

func TestParseCommand(t *testing.T) {
	// non 7-bit ASCII command
	cmd, err := parseCommand("čšěř test@test.te -d")
	assert.Nil(t, cmd, "parsing non 7-bit ASCII command 'čšěř' should return nil command")
	assert.Error(t, err, "parsing non 7-bit ASCII command 'čšěř' should return error")

	// empty command
	cmd, err = parseCommand("")
	assert.Nil(t, cmd, "parsing empty command should return nil command")
	assert.Error(t, err, "parsing empty command should return error")

	// valid command
	cmd, err = parseCommand("MAIL FROM:<test@test.te> 8BITMIME")
	assert.NoError(t, err, "parsing 'MAIL' command shouldn't return error")
	assert.Equal(t, mailCmd, cmd.commandCode, "parsing 'MAIL' command should parse correct command code 'mailCmd'")
	assert.Equal(t, "MAIL", cmd.verb, "parsing 'MAIL' command should parse correct verb 'MAIL'")
	assert.Equal(t, "MAIL FROM:<test@test.te> 8BITMIME", cmd.data, "parsing 'MAIL FROM:<test@test.te> 8BITMIME' should parse 'FROM:<test@test.te> 8BITMIME' as data")
	assert.Equal(t, []string{"FROM:<test@test.te>", "8BITMIME"}, cmd.arguments, "parsing 'MAIL FROM:<test@test.te> 8BITMIME' should parse 'FROM:<test@test.te>', '8BITMIME' as arguments")

	// valid command with no arguments
	cmd, err = parseCommand("RSET")
	assert.NoError(t, err, "parsing 'RSET' command shouldn't return error")
	assert.Equal(t, rsetCmd, cmd.commandCode, "parsing 'RSET' command should parse correct command code 'rsetCmd'")
	assert.Equal(t, "RSET", cmd.verb, "parsing 'RSET' command should parse correct verb 'RSET'")
	assert.Equal(t, "RSET", cmd.data, "parsing 'RSET' command with no arguments should parse empty data")
	assert.Equal(t, []string(nil), cmd.arguments, "parsing 'RSET' command with no arguments should parse empty arguments")

	// invalid command with no arguments
	cmd, err = parseCommand("RSET x")
	assert.Error(t, err, "parsing 'RSET' (command which shouldn't have arguments) command with arguments should return error")
}

func TestCommand_String(t *testing.T) {
	// non 7-bit ASCII command
	orig := "MAIL FROM:<test@test.te> 8BITMIME"
	cmd, _ := parseCommand(orig)
	assert.Equal(t, orig, cmd.String(), "converting cmd back to string should return original parsed string")

	// non 7-bit ASCII command
	orig = "RSET"
	cmd, _ = parseCommand(orig)
	assert.Equal(t, orig, cmd.String(), "converting cmd back to string should return original parsed string")
}
