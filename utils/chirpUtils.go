package utils

import (
	"fmt"
)

func ValidateChirp(chirpBody string) (string, error) {
	if len(chirpBody) == 0 {
		return "", fmt.Errorf("Chirp body cannot be empty")
	}
	if len(chirpBody) > 140 {
		return "", fmt.Errorf("Chirp body cannot exceed 140 characters")
	}
	replacedBody := ReplaceBadWords(chirpBody)
	return replacedBody, nil
}
