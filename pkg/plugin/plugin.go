package plugin

import (
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

func SetupVeth(contNetns ns.NetNS, contIfaceName string, requestedMac string) (*current.Interface, *current.Interface, error) {
	hostIface := &current.Interface{}
	contIface := &current.Interface{}

	err := contNetns.Do(func(hostNetns ns.NetNS) error {
		hostVeth, containerVeth, err := ip.SetupVeth(contIfaceName, 1500, requestedMac, hostNetns)
		if err != nil {
			return err
		}

		if err := setInterfaceUp(contIfaceName); err != nil {
			return err
		}

		contIface.Name = containerVeth.Name
		contIface.Mac = containerVeth.HardwareAddr.String()
		contIface.Sandbox = contNetns.Path()
		hostIface.Name = hostVeth.Name
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	// Refetch the hostIface since its MAC address may change during network namespace move.
	if err = refetchIface(hostIface); err != nil {
		return nil, nil, err
	}

	return hostIface, contIface, nil
}

func setInterfaceUp(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return err
	}

	return nil
}

func refetchIface(iface *current.Interface) error {
	iface.Mac = getHardwareAddr(iface.Name)
	return nil
}

func getHardwareAddr(ifName string) string {
	ifLink, err := netlink.LinkByName(ifName)
	if err != nil {
		return ""
	}
	return ifLink.Attrs().HardwareAddr.String()
}
