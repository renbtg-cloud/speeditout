// Package audioquality implements evidence-limited audio quality advisories.
// It does NOT infer human intent, transcribe speech or diagnose smartphone hardware.
package audioquality

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

type Signal struct {
	RMSdBFS         float64 `json:"rms_dbfs"`
	ClippedFraction float64 `json:"clipped_fraction"`
	DurationSeconds float64 `json:"duration_seconds"`
}

type Request struct {
	Signal              *Signal `json:"signal,omitempty"`
	Transcript          string  `json:"transcript,omitempty"`
	TranscriptionStatus string  `json:"transcription_status"` // unavailable, uncertain, ready
	Platform            string  `json:"platform"`             // android, ios, web, desktop, unknown
	InputRoute          string  `json:"input_route"`          // speakerphone, handset, headset, computer_mic, unknown
	Attempts            int     `json:"attempts"`
}

type Result struct {
	Level                 string   `json:"level"`   // retry, confirm, usable, unknown
	Reasons               []string `json:"reasons"` // diagnostic evidence, not a hardware diagnosis
	Message               string   `json:"message"`
	Suggestions           []string `json:"suggestions,omitempty"`
	Transcript            string   `json:"transcript,omitempty"`
	NeedsUserConfirmation bool     `json:"needs_user_confirmation"`
	AutoSendAllowed       bool     `json:"auto_send_allowed"` // deliberately always false
}

func (r Request) Validate() error {
	if len(r.Transcript) > 8192 {
		return errors.New("transcript too long")
	}
	if r.Attempts < 0 || r.Attempts > 50 {
		return errors.New("attempts out of range")
	}
	valid := func(value string, values ...string) bool {
		for _, item := range values {
			if value == item {
				return true
			}
		}
		return false
	}
	if !valid(r.TranscriptionStatus, "unavailable", "uncertain", "ready") {
		return errors.New("invalid transcription_status")
	}
	if !valid(r.Platform, "android", "ios", "web", "desktop", "unknown") {
		return errors.New("invalid platform")
	}
	if !valid(r.InputRoute, "speakerphone", "handset", "headset", "computer_mic", "unknown") {
		return errors.New("invalid input_route")
	}
	if r.Signal != nil {
		s := r.Signal
		if math.IsNaN(s.RMSdBFS) || math.IsInf(s.RMSdBFS, 0) || s.RMSdBFS > 0 || s.RMSdBFS < -200 {
			return errors.New("invalid rms_dbfs")
		}
		if math.IsNaN(s.ClippedFraction) || math.IsInf(s.ClippedFraction, 0) || s.ClippedFraction < 0 || s.ClippedFraction > 1 {
			return errors.New("invalid clipped_fraction")
		}
		if math.IsNaN(s.DurationSeconds) || math.IsInf(s.DurationSeconds, 0) || s.DurationSeconds <= 0 || s.DurationSeconds > 120 {
			return errors.New("invalid duration_seconds")
		}
	}
	return nil
}

func Assess(req Request) (Result, error) {
	if err := req.Validate(); err != nil {
		return Result{}, fmt.Errorf("invalid audio assessment: %w", err)
	}
	transcript := strings.TrimSpace(req.Transcript)
	result := Result{Level: "unknown", Transcript: transcript, AutoSendAllowed: false, NeedsUserConfirmation: true}
	if req.Signal != nil {
		if req.Signal.RMSdBFS < -45 {
			result.Reasons = append(result.Reasons, "signal_very_quiet")
		}
		if req.Signal.ClippedFraction >= 0.01 {
			result.Reasons = append(result.Reasons, "signal_clipped")
		}
		if req.Signal.DurationSeconds < 0.25 {
			result.Reasons = append(result.Reasons, "recording_too_short")
		}
	}
	if req.TranscriptionStatus == "uncertain" {
		result.Reasons = append(result.Reasons, "transcription_uncertain")
	}
	if req.TranscriptionStatus == "unavailable" {
		result.Reasons = append(result.Reasons, "transcription_unavailable")
	}
	poorSignal := false
	for _, reason := range result.Reasons {
		if reason == "signal_very_quiet" || reason == "signal_clipped" || reason == "recording_too_short" {
			poorSignal = true
		}
	}
	switch {
	case poorSignal || req.TranscriptionStatus == "unavailable":
		result.Level = "retry"
		if transcript != "" {
			result.Message = "Não tenho certeza de que captei seu áudio direito. Entendi: \"" + transcript + "\". Foi isso?"
		} else {
			result.Message = "Não consegui obter uma transcrição confiável desse áudio. Quer tentar novamente ou digitar?"
		}
	case req.TranscriptionStatus == "uncertain":
		result.Level = "confirm"
		if transcript != "" {
			result.Message = "Posso ter entendido errado. O que captei foi: \"" + transcript + "\". Acertei?"
		} else {
			result.Message = "Não consegui compreender o áudio com segurança. Pode repetir ou escrever?"
		}
	case req.TranscriptionStatus == "ready" && transcript != "":
		result.Level = "usable"
		result.NeedsUserConfirmation = false
		result.Message = "Transcrição disponível para revisão."
	default:
		result.Message = "Ainda não há informação suficiente para avaliar a captura."
	}
	if result.Level == "retry" || result.Level == "confirm" {
		result.Suggestions = []string{"Se preferir, você também pode corrigir o texto ou digitar."}
		if req.Attempts >= 2 {
			switch req.InputRoute {
			case "speakerphone":
				result.Suggestions = append(result.Suggestions, "Se estiver usando o viva-voz, experimentar o modo telefone pode ajudar. Quer tentar?")
			case "computer_mic":
				result.Suggestions = append(result.Suggestions, "Falar um pouco mais perto do microfone do computador talvez ajude.")
			case "headset":
				result.Suggestions = append(result.Suggestions, "Você pode verificar se o microfone do headset está bem posicionado, perto da boca?")
			default:
				result.Suggestions = append(result.Suggestions, "Se você tiver um headset com microfone perto da boca, pode valer a pena experimentar.")
			}
		}
	}
	return result, nil
}
