package plugin

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"

	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
)

func PrepareLink(contNetns ns.NetNS, hostIfName, contIfName string) (*current.Interface, error) {
	iface := &current.Interface{}

	// 1) Find host-side interface
	link, err := waitLink(hostIfName, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to find host interface %s: %w", hostIfName, err)
	}

	// 2) Must be DOWN for OVS internal ports
	if err := netlink.LinkSetDown(link); err != nil {
		return nil, fmt.Errorf("failed to set %s down: %w", hostIfName, err)
	}

	// 3) Open the real netns path (same semantics as `ip link set ... netns`)
	nsPath := contNetns.Path()
	nsFile, err := os.Open(nsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open target netns path %q: %w", nsPath, err)
	}
	defer nsFile.Close()

	// 4) Try the netlink move using the namespace fd
	if err := netlink.LinkSetNsFd(link, int(nsFile.Fd())); err != nil {
		// If this fails with an error, try fallback to ip right away
		_ = tryIpFallbackMove(hostIfName, nsPath)
		// Re-check where the interface is
	}

	// 5) verify that link moved; if not, fall back to ip
	time.Sleep(150 * time.Millisecond)
	if stillInRoot(hostIfName) {
		// Fallback: call 'ip link set <hostIfName> netns <nsIdentifier>'
		if err := tryIpFallbackMove(hostIfName, nsPath); err != nil {
			return nil, fmt.Errorf("failed to move interface via netlink and ip fallback: %w", err)
		}
	}

	// 6) Now operate inside target netns
	err = contNetns.Do(func(_ ns.NetNS) error {
		// In some kernels the moved link keeps the old name until renamed
		l, err := waitLink(hostIfName, 1*time.Second)
		if err != nil {
			// try the new name too (maybe already renamed)
			l, err = waitLink(contIfName, 1*time.Second)
			if err != nil {
				return fmt.Errorf("moved interface not found by neither %s nor %s: %w", hostIfName, contIfName, err)
			}
		}

		// rename if needed
		if l.Attrs().Name != contIfName {
			if err := netlink.LinkSetName(l, contIfName); err != nil {
				return fmt.Errorf("failed to rename %s -> %s: %w", l.Attrs().Name, contIfName, err)
			}
			// re-fetch with new name
			l, err = waitLink(contIfName, 1*time.Second)
			if err != nil {
				return fmt.Errorf("failed to refetch renamed link %s: %w", contIfName, err)
			}
		}

		// bring up
		if err := netlink.LinkSetUp(l); err != nil {
			return fmt.Errorf("failed to bring %s up: %w", contIfName, err)
		}

		iface.Name = contIfName
		iface.Sandbox = contNetns.Path()
		iface.Mac = l.Attrs().HardwareAddr.String()
		return nil
	})

	if err != nil {
		return nil, err
	}
	return iface, nil
}

// stillInRoot returns true if interface still exists in root netns
func stillInRoot(name string) bool {
	_, err := netlink.LinkByName(name)
	return err == nil
}

// tryIpFallbackMove runs `ip link set <ifname> netns <nsIdentifier>`
// nsPath is contNetns.Path(); this function tries to derive either a PID or a netns name.
func tryIpFallbackMove(ifname, nsPath string) error {
	// If nsPath is /proc/<pid>/ns/net -> use the pid as identifier
	reProc := regexp.MustCompile(`^/proc/(\d+)/ns/net$`)
	if m := reProc.FindStringSubmatch(nsPath); m != nil {
		pid := m[1]
		cmd := exec.Command("ip", "link", "set", "dev", ifname, "netns", pid)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("ip fallback (pid) failed: %v - %s", err, string(out))
		}
		return nil
	}

	// If nsPath is /var/run/netns/<name> or /run/netns/<name> -> extract name and use ip netns
	reRun := regexp.MustCompile(`/(?:run|var/run)/netns/([^/]+)$`)
	if m := reRun.FindStringSubmatch(nsPath); m != nil {
		name := m[1]
		cmd := exec.Command("ip", "link", "set", "dev", ifname, "netns", name)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("ip fallback (name) failed: %v - %s", err, string(out))
		}
		return nil
	}

	// Last resort: call ip with the full path (some ip versions accept file path)
	cmd := exec.Command("ip", "link", "set", "dev", ifname, "netns", nsPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip fallback (path) failed: %v - %s", err, string(out))
	}
	return nil
}

func waitLink(name string, timeout time.Duration) (netlink.Link, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		link, err := netlink.LinkByName(name)
		if err == nil {
			return link, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, fmt.Errorf("timeout waiting for link %s", name)
}
