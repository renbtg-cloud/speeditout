package audioquality

import (
	"bytes"
	"encoding/binary"
	"math"
	"strings"
	"sync"
	"testing"
)

func base() Request {
	return Request{TranscriptionStatus: "ready", Platform: "android", InputRoute: "unknown", Attempts: 1, Transcript: "Oi, tudo bem?"}
}
func contains(xs []string, item string) bool {
	for _, v := range xs {
		if v == item {
			return true
		}
	}
	return false
}
func TestAssessMatrix(t *testing.T) {
	tests := []struct {
		name        string
		modify      func(*Request)
		level       string
		reason      string
		snippet     string
		suggestions int
	}{
		{"usable", func(*Request) {}, "usable", "", "Transcrição disponível", 0},
		{"uncertain text", func(r *Request) { r.TranscriptionStatus = "uncertain" }, "confirm", "transcription_uncertain", "Oi, tudo bem?", 1},
		{"unavailable no text", func(r *Request) { r.TranscriptionStatus = "unavailable"; r.Transcript = "" }, "retry", "transcription_unavailable", "Quer tentar", 1},
		{"unavailable partial", func(r *Request) { r.TranscriptionStatus = "unavailable" }, "retry", "transcription_unavailable", "Oi, tudo bem?", 1},
		{"no transcript", func(r *Request) { r.Transcript = "" }, "unknown", "", "informação suficiente", 0},
		{"clipped", func(r *Request) { r.Signal = &Signal{RMSdBFS: -12, ClippedFraction: 0.025, DurationSeconds: 4} }, "retry", "signal_clipped", "Oi, tudo bem?", 1},
		{"border clipped", func(r *Request) { r.Signal = &Signal{RMSdBFS: -12, ClippedFraction: 0.01, DurationSeconds: 4} }, "retry", "signal_clipped", "Oi, tudo bem?", 1},
		{"not clipped", func(r *Request) { r.Signal = &Signal{RMSdBFS: -12, ClippedFraction: 0.0099, DurationSeconds: 4} }, "usable", "", "Transcrição", 0},
		{"quiet", func(r *Request) { r.Signal = &Signal{RMSdBFS: -52, ClippedFraction: 0, DurationSeconds: 4} }, "retry", "signal_very_quiet", "Oi, tudo bem?", 1},
		{"short", func(r *Request) { r.Signal = &Signal{RMSdBFS: -15, ClippedFraction: 0, DurationSeconds: 0.19} }, "retry", "recording_too_short", "Oi, tudo bem?", 1},
		{"barely long", func(r *Request) { r.Signal = &Signal{RMSdBFS: -15, ClippedFraction: 0, DurationSeconds: 0.25} }, "usable", "", "Transcrição", 0},
		{"speakerphone first", func(r *Request) { r.TranscriptionStatus = "uncertain"; r.InputRoute = "speakerphone" }, "confirm", "transcription_uncertain", "Oi, tudo bem?", 1},
		{"speakerphone repeat", func(r *Request) { r.TranscriptionStatus = "uncertain"; r.InputRoute = "speakerphone"; r.Attempts = 3 }, "confirm", "transcription_uncertain", "Oi, tudo bem?", 2},
		{"headset repeat", func(r *Request) { r.TranscriptionStatus = "uncertain"; r.InputRoute = "headset"; r.Attempts = 3 }, "confirm", "transcription_uncertain", "Oi, tudo bem?", 2},
		{"computer repeat", func(r *Request) {
			r.TranscriptionStatus = "uncertain"
			r.InputRoute = "computer_mic"
			r.Platform = "desktop"
			r.Attempts = 2
		}, "confirm", "transcription_uncertain", "Oi, tudo bem?", 2},
		{"unknown repeat", func(r *Request) { r.TranscriptionStatus = "uncertain"; r.InputRoute = "unknown"; r.Attempts = 2 }, "confirm", "transcription_uncertain", "Oi, tudo bem?", 2},
		{"personal speech intact", func(r *Request) { r.TranscriptionStatus = "uncertain"; r.Transcript = "Porra, que merda!" }, "confirm", "transcription_uncertain", "Porra, que merda!", 1},
		{"false strong confidence never", func(r *Request) {
			r.TranscriptionStatus = "ready"
			r.Signal = &Signal{RMSdBFS: -80, DurationSeconds: 10}
		}, "retry", "signal_very_quiet", "Oi, tudo bem?", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := base()
			tt.modify(&r)
			out, err := Assess(r)
			if err != nil {
				t.Fatal(err)
			}
			if out.Level != tt.level || !contains(out.Reasons, tt.reason) && tt.reason != "" || !strings.Contains(out.Message, tt.snippet) || len(out.Suggestions) != tt.suggestions {
				t.Fatalf("got %+v", out)
			}
			if out.AutoSendAllowed {
				t.Fatal("must never auto-send")
			}
		})
	}
}
func TestInvalidRequests(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Request)
	}{
		{"unknown ASR", func(r *Request) { r.TranscriptionStatus = "certain" }},
		{"unknown platform", func(r *Request) { r.Platform = "spaceship" }},
		{"unknown route", func(r *Request) { r.InputRoute = "airpods?" }},
		{"negative attempts", func(r *Request) { r.Attempts = -1 }},
		{"too many attempts", func(r *Request) { r.Attempts = 51 }},
		{"overlong transcript", func(r *Request) { r.Transcript = strings.Repeat("a", 8193) }},
		{"positive RMS", func(r *Request) { r.Signal = &Signal{RMSdBFS: 0.1, DurationSeconds: 1} }},
		{"invalid clipping", func(r *Request) { r.Signal = &Signal{RMSdBFS: -12, ClippedFraction: 1.2, DurationSeconds: 1} }},
		{"invalid duration", func(r *Request) { r.Signal = &Signal{RMSdBFS: -12, DurationSeconds: 0} }},
		{"long duration", func(r *Request) { r.Signal = &Signal{RMSdBFS: -12, DurationSeconds: 121} }},
		{"nan", func(r *Request) { r.Signal = &Signal{RMSdBFS: math.NaN(), DurationSeconds: 1} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := base()
			tc.change(&r)
			if _, err := Assess(r); err == nil {
				t.Fatalf("accepted invalid request: %+v", r)
			}
		})
	}
}
func TestPCMAndWAV(t *testing.T) {
	zero := make([]byte, 16000*2)
	s, err := MeasurePCM16LE(zero, 16000)
	if err != nil || s.RMSdBFS != -200 || s.ClippedFraction != 0 || s.DurationSeconds != 1 {
		t.Fatalf("silence: %+v %v", s, err)
	}
	peak := make([]byte, 16000*2)
	for i := 0; i < 16000; i++ {
		binary.LittleEndian.PutUint16(peak[2*i:], uint16(32767))
	}
	p, err := MeasurePCM16LE(peak, 16000)
	if err != nil || p.ClippedFraction != 1 || p.RMSdBFS > 0 {
		t.Fatalf("clipped: %+v %v", p, err)
	}
	if _, err = MeasurePCM16LE([]byte{42}, 16000); err == nil {
		t.Fatal("odd byte count accepted")
	}
	if _, err = MeasurePCM16LE(peak, 0); err == nil {
		t.Fatal("invalid sampling accepted")
	}
	var wav bytes.Buffer
	wav.WriteString("RIFF")
	binary.Write(&wav, binary.LittleEndian, uint32(36+len(peak)))
	wav.WriteString("WAVEfmt ")
	binary.Write(&wav, binary.LittleEndian, uint32(16))
	binary.Write(&wav, binary.LittleEndian, uint16(1))
	binary.Write(&wav, binary.LittleEndian, uint16(1))
	binary.Write(&wav, binary.LittleEndian, uint32(16000))
	binary.Write(&wav, binary.LittleEndian, uint32(32000))
	binary.Write(&wav, binary.LittleEndian, uint16(2))
	binary.Write(&wav, binary.LittleEndian, uint16(16))
	wav.WriteString("data")
	binary.Write(&wav, binary.LittleEndian, uint32(len(peak)))
	wav.Write(peak)
	got, err := WAVPCM16Mono(bytes.NewReader(wav.Bytes()))
	if err != nil || got != p {
		t.Fatalf("WAV mismatch: %+v %v", got, err)
	}
	if _, err = WAVPCM16Mono(strings.NewReader("invalid")); err == nil {
		t.Fatal("bad WAV accepted")
	}
}
func TestConcurrentAssess(t *testing.T) {
	const N = 32
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := base()
			if i%2 == 0 {
				r.TranscriptionStatus = "uncertain"
			}
			for j := 0; j < 100; j++ {
				out, err := Assess(r)
				if err != nil || out.AutoSendAllowed {
					t.Errorf("%v %+v", err, out)
					return
				}
			}
		}(i)
	}
	wg.Wait()
}
