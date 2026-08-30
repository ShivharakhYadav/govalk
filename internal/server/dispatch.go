package server

import (
	"fmt"
	"strings"

	"github.com/ShivharakhYadav/govalk/internal/resp"
)

// CommandFunc implements one command's behavior, given its arguments
// (excluding the command name). Arity has already been checked by the
// Dispatcher before a CommandFunc is invoked.
type CommandFunc func(args [][]byte) resp.Value

type commandSpec struct {
	minArgs int // inclusive
	maxArgs int // inclusive; -1 means unbounded
	fn      CommandFunc
}

// Dispatcher routes decoded requests to registered command handlers,
// producing Redis-style wrong-arity and unknown-command errors for
// anything that doesn't match a registered command's signature.
//
// Concurrency: Register is meant to be called during single-goroutine
// server setup, before Serve begins accepting connections. Dispatch is
// then called concurrently from many connection goroutines, but only
// ever reads the commands map - concurrent reads with no concurrent
// writes are safe for Go maps. Calling Register after Serve has started
// would be a data race; nothing in this package does that.
type Dispatcher struct {
	commands map[string]commandSpec
}

// NewDispatcher returns an empty Dispatcher ready for Register calls.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{commands: make(map[string]commandSpec)}
}

// Register adds a command handler under name (case-insensitive).
// minArgs/maxArgs bound the argument count, excluding the command name
// itself; maxArgs = -1 means no upper bound.
func (d *Dispatcher) Register(name string, minArgs, maxArgs int, fn CommandFunc) {
	d.commands[strings.ToUpper(name)] = commandSpec{minArgs: minArgs, maxArgs: maxArgs, fn: fn}
}

// Dispatch is the RequestHandler (see conn.go) wired into the server: it
// converts a decoded request Value into a Command, looks up its
// registered handler, checks arity, and invokes it - or produces a
// Redis-style error reply if any step fails.
func (d *Dispatcher) Dispatch(req resp.Value) resp.Value {
	cmd, err := commandFromValue(req)
	if err != nil {
		return resp.NewError("ERR " + err.Error())
	}

	spec, ok := d.commands[cmd.Name]
	if !ok {
		return resp.NewError(fmt.Sprintf("ERR unknown command '%s'", cmd.Name))
	}

	if len(cmd.Args) < spec.minArgs || (spec.maxArgs >= 0 && len(cmd.Args) > spec.maxArgs) {
		return resp.NewError(fmt.Sprintf(
			"ERR wrong number of arguments for '%s' command", strings.ToLower(cmd.Name)))
	}

	return spec.fn(cmd.Args)
}
