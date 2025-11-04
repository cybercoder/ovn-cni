package main

import (
	"fmt"
	"log"
	"os"
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
	log.Printf("cni ns: %s", args.Netns)
	// mac := "c0:ff:ee:00:00:11"
	ifName := "port1"
	contIfName := args.IfName
	if err := ovsClient.AddPort("br-int", ifName, "internal"); err != nil {
		log.Printf("Error adding port to ovs: %v", err)
		return err
	}

	contNetns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to open netns %q: %v", args.Netns, err)
	}
	defer contNetns.Close()

	iface, err := plugin.PrepareLink(contNetns, ifName, contIfName)
	if err != nil {
		log.Printf("%v", err)
		return err
	}

	result := &current.Result{
		Interfaces: []*current.Interface{iface},
		CNIVersion: version.Current(),
	}
	return cnitypes.PrintResult(result, version.Current())
}

func cmdDel(args *skel.CmdArgs) error {
	ovsClient, err := ovs.CreateOVSclient()
	if err != nil {
		log.Printf("error on creating ovs client: %v", err)
		return err
	}
	err = ovsClient.DelPort("br-int", "port1")
	if err != nil {
		log.Printf("Error on deleting port %s from ovs: %v", "port1", err)
		return err
	}
	return nil
}

func main() {
	runtime.LockOSThread()
	f, err := os.OpenFile("/var/log/ik8s-ovn-cni", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)
	// os.Stdout = f
	//os.Stderr = f

	funcs := skel.CNIFuncs{
		Add: cmdAdd,
		Del: cmdDel,
	}
	skel.PluginMainFuncs(funcs, version.All, "ovn-cni")
}
