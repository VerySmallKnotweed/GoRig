package inventory

import (
	"net"
	"reflect"
	"testing"
)

func TestIPAddressesByMAC(t *testing.T) {
	mac, _ := net.ParseMAC("aa:bb:cc:dd:ee:ff")
	iface := net.Interface{HardwareAddr: mac}
	parse := func(s string) net.Addr { ip, subnet, _ := net.ParseCIDR(s); subnet.IP = ip; return subnet }
	got := ipAddressesByMAC([]net.Interface{iface}, func(net.Interface) ([]net.Addr, error) {
		return []net.Addr{parse("192.168.1.2/24"), parse("2001:db8::1/64"), parse("192.168.1.2/24")}, nil
	})
	want := map[string][]string{"AA:BB:CC:DD:EE:FF": {"192.168.1.2", "2001:db8::1"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}
}
