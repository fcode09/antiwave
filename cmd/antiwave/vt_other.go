//go:build !windows

package main

// enableVT is a no-op outside Windows.
func enableVT() {}
