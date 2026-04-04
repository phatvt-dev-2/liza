// Package main is a minimal binary used by MCP handler tests.
// When built as "liza" and invoked with "agent", it matches
// the isLizaAgentProcess identity check (argv[0]="liza", argv[1]="agent").
// It sleeps until killed.
package main

import "time"

func main() { time.Sleep(time.Hour) }
