package system

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now().UTC() }

type IDGenerator struct{}

func NewIDGenerator() IDGenerator { return IDGenerator{} }

func (IDGenerator) NewID(prefix string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strings.TrimSuffix(prefix+"_"+time.Now().UTC().Format("20060102150405.000000000"), ".")
	}
	if prefix == "" {
		return hex.EncodeToString(b[:])
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}
