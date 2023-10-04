package metallb

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHelperFunctions(t *testing.T) {
	network := net.IPNet{
		IP:   net.IPv4(192, 168, 1, 0),
		Mask: net.IPv4Mask(255, 255, 255, 0),
	}
	// this should choose the upper half of the input network, minus network and broadcast addresses
	// since we start with 192.168.1.0/24, we should get 192.168.1.128/25. the complete range is
	// 192.168.1.128-192.168.1.255, and the returned range is thus 192.168.1.129-192.168.1.254.
	ip1, ip2 := getIPRangeForMetallb(network)
	assert.Equal(t, net.IPv4(192, 168, 1, 129).String(), ip1.String())
	assert.Equal(t, net.IPv4(192, 168, 1, 254).String(), ip2.String())
}
