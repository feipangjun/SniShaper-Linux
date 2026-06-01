package main

import "fmt"

func writeCoreMarker(execDir, name, detail string) {
}

func markerDetail(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
