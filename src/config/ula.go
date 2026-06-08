package config

import (
	"crypto/rand"
	"fmt"
	"net"
)

// GenerateRandomULAPrefix generates a random IPv6 Unique Local Address (ULA) prefix.
// The prefix is in the form fc00::/48 or fd00::/48 (a common convention).
// The first 8 bits are 1111 110x (FC or FD), and the next 40 bits are random.
func GenerateRandomULAPrefix(size int) string {
	var ip = make(net.IP, 16)

	rand.Read(ip)

	// Set the first byte to FC or FD
	// 1111 110x means the first 7 bits are fixed, 8th bit is random
	// 0xFC = 1111 1100, 0xFD = 1111 1101
	ip[0] = 0xFC | (ip[0] & 0x01)

	ip = ip.Mask(net.CIDRMask(size, 128))

	return fmt.Sprintf("%s/%d", ip.To16().String(), size)
}
