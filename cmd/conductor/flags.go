package main

import (
	"github.com/integrii/flaggy"
)

type HookFunc func() error

func (f HookFunc) Run() error {
	return f()
}

func Hook(f HookFunc) flaggy.CommandHook {
	return f
}
