package carrot

import (
	"fmt"
	"math/rand"
)

var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyz")
var numberRunes = []rune("0123456789")

func randRunes(n int, source []rune) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = source[rand.Intn(len(source))]
	}
	return string(b)
}

func RandText(n int) string {
	return randRunes(n, letterRunes)
}

func RandNumberText(n int) string {
	return randRunes(n, numberRunes)
}

func FormatSizeHuman(size float64) string {
	if size <= 0 {
		return "0 B"
	}
	if size < 1024 {
		return fmt.Sprintf("%.0f B", size)
	}
	size = size / 1024
	if size < 1024 {
		return fmt.Sprintf("%.1f KB", size)
	}
	size = size / 1024
	if size < 1024 {
		return fmt.Sprintf("%.1f MB", size)
	}
	size = size / 1024
	return fmt.Sprintf("%.1f GB", size)
}
