package pagination

import (
	"encoding/base64"
)

// EncodeCursor builds an opaque pagination token from a document ID.
func EncodeCursor(docID string) string {
	return base64.URLEncoding.EncodeToString([]byte(docID))
}

// DecodeCursor reverses EncodeCursor.
func DecodeCursor(s string) (string, error) {
	raw, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
