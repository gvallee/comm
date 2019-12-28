package util

import (
	"math/rand"
	"time"
)

// GenerateID creates a 256-character long random ID
func GenerateID() string {
	// generate a 256 random character string
	rand.Seed(time.Now().UnixNano())
	digits := "0123456789"
	all := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz" + digits
	length := 256
	buf := make([]byte, length)
	buf[0] = digits[rand.Intn(len(digits))]
	for i := 2; i < length; i++ {
		buf[i] = all[rand.Intn(len(all))]
	}
	rand.Shuffle(len(buf), func(i, j int) {
		buf[i], buf[j] = buf[j], buf[i]
	})

	return string(buf)
}
