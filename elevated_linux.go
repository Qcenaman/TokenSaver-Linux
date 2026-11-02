//go:build linux

package main

import (
	"errors"
	"fmt"
	"io"
)

var errElevWindowsOnly = errors.New("the admin helper isn't installed yet (tokensaver install-helper)")

func startElevated(req elevRequest, out io.Writer) (*elevProc, error) { return nil, errElevWindowsOnly }

func serveElevatedHelper() int { fmt.Println(errElevWindowsOnly); return 1 }

func consoleInstallHelper() { fmt.Println(errElevWindowsOnly) }

func consoleUninstallHelper() { fmt.Println(errElevWindowsOnly) }

func helperStatus() string { return "not available" }

func refreshHelper() {}

func moveTree(src, dst string) error { return errElevWindowsOnly }
