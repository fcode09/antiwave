//go:build !windows

package main

// enableVT es un no-op fuera de Windows.
func enableVT() {}
