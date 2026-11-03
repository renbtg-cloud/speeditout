// Command memorycompact rewrites the local JSONL journal without tombstoned
// feedback. Operator-only: no HTTP endpoint, and never promises disk erasure.
package main

import (
	"flag"
	"fmt"
	"github.com/renbtg-cloud/speeditout/engine"
	"os"
)

func main() {
	path := flag.String("path", "", "existing JSONL journal")
	apply := flag.Bool("apply", false, "perform atomic replacement; omit to inspect only")
	flag.Parse()
	if *path == "" {
		fmt.Fprintln(os.Stderr, "-path is required")
		os.Exit(2)
	}
	if _, err := os.Stat(*path); err != nil {
		fmt.Fprintln(os.Stderr, "journal must already exist:", err)
		os.Exit(2)
	}
	m, err := engine.NewMemory(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer m.Close()
	if !*apply {
		fmt.Println("DRY RUN — no data changed; -apply rewrites ACTIVE journal (not backups or SSD sectors)")
		return
	}
	result, err := m.CompactJournal()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("active journal compacted: bytes %d -> %d, preferences=%d, examples=%d, feedback=%d\n", result.BeforeBytes, result.AfterBytes, result.Preferences, result.Examples, result.ActiveFeedback)
	fmt.Println("NOT secure erasure: backup copies and physical storage may retain earlier contents")
}
