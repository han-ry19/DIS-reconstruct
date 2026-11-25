package domain

import (
	"crypto/rand"
	"fmt"
	"regexp"
)

var dataIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\.data$`)

// NewDataID generates a random UUIDv4 in the data zone before request signing.
func NewDataID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x.data", value[:4], value[4:6], value[6:8], value[8:10], value[10:]), nil
}

func ValidDataID(id string) bool { return dataIDPattern.MatchString(id) }
