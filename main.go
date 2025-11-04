package main

import (
	"fmt"
	"log"
	"runtime"

	"github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/version"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/cybercoder/ovn-cni/pkg/ovs"
	"github.com/cybercoder/ovn-cni/pkg/plugin"
)

func cmdAdd(args *skel.CmdArgs) error {
	ovsClient, err := ovs.CreateOVSclient()
	if err != nil {
		return err
	}
	contNetns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer contNetns.Close()
	mac := "c0:ff:ee:00:00:11"
	hostIface, contIface, err := plugin.SetupVeth(contNetns, args.IfName, mac)
	if err != nil {
		return err
	}
	if err := ovsClient.AddPort("br-int", hostIface.Name, "access"); err != nil {
		log.Printf("Error adding port to ovs: %v", err)
		return err
	}
	result := &current.Result{
		Interfaces: []*current.Interface{hostIface, contIface},
		CNIVersion: version.Current(),
	}
	return cnitypes.PrintResult(result, version.Current())
}

func cmdDel(args *skel.CmdArgs) error {
	return nil
}

func main() {
	runtime.LockOSThread()

	funcs := skel.CNIFuncs{
		Add: cmdAdd,
		Del: cmdDel,
	}
	skel.PluginMainFuncs(funcs, version.All, "ovn-cni")
}
