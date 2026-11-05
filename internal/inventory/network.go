package inventory

import (
	"net"
	"sort"
	"strings"
)

// ipAddressesByMAC is a point-in-time observation; disconnected interfaces
// and address enumeration errors may yield no IP address.
func ipAddressesByMAC(interfaces []net.Interface, addrs func(net.Interface) ([]net.Addr, error)) map[string][]string {
	sets := map[string]map[string]bool{}
	for _, iface := range interfaces {
		mac := strings.ToUpper(iface.HardwareAddr.String())
		if mac == "" {
			continue
		}
		addresses, err := addrs(iface)
		if err != nil {
			continue
		}
		for _, addr := range addresses {
			ip, _, err := net.ParseCIDR(addr.String())
			if err != nil || ip == nil {
				continue
			}
			if sets[mac] == nil {
				sets[mac] = map[string]bool{}
			}
			sets[mac][ip.String()] = true
		}
	}
	out := map[string][]string{}
	for mac, addresses := range sets {
		for ip := range addresses {
			out[mac] = append(out[mac], ip)
		}
		sort.Strings(out[mac])
	}
	return out
}
