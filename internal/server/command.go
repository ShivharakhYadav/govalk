package server

import (
	"errors"
	"strings"

	"github.com/ShivharakhYadav/govalk/internal/resp"
)

var (
	errNotAnArray      = errors.New("expected a command to be a RESP array")
	errEmptyCommand    = errors.New("empty command")
	errNonBulkArgument = errors.New("command arguments must be bulk strings")
)

// Command is a parsed client request: an uppercase-normalized command
// name and its raw argument bytes (the command name itself is not part
// of Args).
type Command struct {
	Name string
	Args [][]byte
}

// commandFromValue converts a decoded request Value into a Command. The
// wire-level RESP syntax (P2) is more general than "array of bulk
// strings" on purpose - it's reused for AOF records and arbitrary
// replies too - so the "a command must be a non-null array of non-null
// bulk strings" constraint belongs here, at the command layer, not in
// the reader. Any shape that doesn't satisfy it is reported as an error,
// exactly like an unknown command or wrong arity: a RESP error reply,
// never a dropped connection.
func commandFromValue(v resp.Value) (Command, error) {
	if v.Kind != resp.Array || v.Null {
		return Command{}, errNotAnArray
	}
	if len(v.Elems) == 0 {
		return Command{}, errEmptyCommand
	}
	args := make([][]byte, len(v.Elems))
	for i, e := range v.Elems {
		if e.Kind != resp.BulkString || e.Null {
			return Command{}, errNonBulkArgument
		}
		args[i] = e.Bulk
	}
	return Command{
		Name: strings.ToUpper(string(args[0])),
		Args: args[1:],
	}, nil
}
