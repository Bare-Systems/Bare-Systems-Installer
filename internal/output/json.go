package output

import (
	"encoding/json"
	"io"
	"time"

	apperrors "github.com/Bare-Systems/Bare-Systems-Installer/internal/errors"
)

func JSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

type Envelope struct {
	OK        bool            `json:"ok"`
	Command   string          `json:"command"`
	Code      apperrors.Code  `json:"code"`
	Message   string          `json:"message"`
	Data      any             `json:"data"`
	Warnings  []string        `json:"warnings"`
	Errors    []EnvelopeError `json:"errors"`
	Timestamp string          `json:"timestamp"`
}

type EnvelopeError struct {
	Message string `json:"message"`
}

type Clock func() time.Time

func NewEnvelope(command string, code apperrors.Code, message string, data any, clock Clock) Envelope {
	if clock == nil {
		clock = time.Now
	}

	return Envelope{
		OK:        code == apperrors.CodeOK,
		Command:   command,
		Code:      code,
		Message:   message,
		Data:      data,
		Warnings:  []string{},
		Errors:    errorsFor(code, message),
		Timestamp: clock().UTC().Format(time.RFC3339),
	}
}

func WriteEnvelope(w io.Writer, command string, code apperrors.Code, message string, data any, clock Clock) error {
	return JSON(w, NewEnvelope(command, code, message, data, clock))
}

func errorsFor(code apperrors.Code, message string) []EnvelopeError {
	if code == apperrors.CodeOK {
		return []EnvelopeError{}
	}
	return []EnvelopeError{{Message: message}}
}
