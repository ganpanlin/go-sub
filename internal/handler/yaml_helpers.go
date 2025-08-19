package handler

import (
	"bytes"
	"go-sub/internal/converter"
	"strconv"
)

// marshalYAML encodes and then un-escapes unicode (emoji stay readable)
// Kept for simulate.go and other handlers that need raw YAML output.
func marshalYAML(v interface{}) ([]byte, error) {
	return converter.MarshalClashYAML(v)
}

// unescapeYAMLUnicode converts \Uxxxxxxxx back to UTF-8 characters
func unescapeYAMLUnicode(data []byte) []byte {
	var out bytes.Buffer
	i := 0
	for i < len(data) {
		if i+10 <= len(data) && data[i] == '\\' && data[i+1] == 'U' {
			hex := string(data[i+2 : i+10])
			if code, err := strconv.ParseInt(hex, 16, 32); err == nil {
				out.WriteString(string(rune(code)))
				i += 10
				continue
			}
		}
		out.WriteByte(data[i])
		i++
	}
	return out.Bytes()
}
