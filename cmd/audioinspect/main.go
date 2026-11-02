package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"github.com/renbtg-cloud/speeditout/audioquality"
)

func main() {
	input := flag.String("wav", "", "mono PCM16 WAV to inspect")
	flag.Parse()
	if *input == "" {
		fmt.Fprintln(os.Stderr, "usage: audioinspect -wav FILE.wav")
		os.Exit(2)
	}
	f, err := os.Open(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	signal, err := audioquality.WAVPCM16Mono(f)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err = enc.Encode(signal); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
