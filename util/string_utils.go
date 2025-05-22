package util

// InsertStringConcat insert string at position
func InsertStringConcat(text string, n int, s string) string {
	runes := []rune(text)
	if n < 0 || n > len(runes) {
		return text
	}
	result := make([]rune, len(runes)+len([]rune(s)))
	copy(result[:n], runes[:n])
	copy(result[n:], []rune(s))
	copy(result[n+len([]rune(s)):], runes[n:])
	return string(result)
}
