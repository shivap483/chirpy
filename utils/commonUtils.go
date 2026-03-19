package utils

import (
	"strings"
)

func ReplaceBadWords(input string) string {
	badWords := []string{"kerfuffle", "sharbert", "fornax"}
	replacement := "****"

	for _, badWord := range badWords {
		input = replaceAll(input, badWord, replacement)
	}
	return input
}

func replaceAll(input, old, new string) string {
	for {
		index := indexOf(input, old)
		if index == -1 {
			break
		}
		input = input[:index] + new + input[index+len(old):]
	}
	return input
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if strings.EqualFold(s[i:i+len(substr)], substr) {
			return i
		}
	}
	return -1
}
