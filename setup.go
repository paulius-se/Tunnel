package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
)

const (
	tunInputIPAdr    = "10.0.0.1"
	tunOutputIPAdr   = "10.0.0.2"
	outgoingNetIface = "enp0s3"
	routingTableName = "Tun"
)

func runCmd(cmdPath string, args ...string) {
	cmd := exec.Command(cmdPath, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err := cmd.Run()
	if nil != err {
		log.Fatalln(fmt.Sprintf("Error running %s:", cmdPath), err)
	}
}

func setup(ifaceName string) {
	fmt.Println("Setup interface: ", ifaceName)
	os := runtime.GOOS
	switch os {
	case "linux":
		// setup for linux
		runCmd("/sbin/ifconfig", ifaceName, "inet", tunInputIPAdr+"/30", "up")
		runCmd("/sbin/sysctl", "-w", "net.ipv4.ip_forward=1")
		runCmd("/sbin/ip", "route", "add", "default", "dev", ifaceName, "table", routingTableName)
		runCmd("/sbin/ip", "rule", "add", "from", "all", "lookup", routingTableName)
		runCmd("/sbin/ip", "rule", "add", "from", tunOutputIPAdr, "lookup", "main", "priority", "500")
		runCmd("/sbin/iptables", "-t", "nat", "-A", "POSTROUTING", "-o", outgoingNetIface, "-s", tunOutputIPAdr, "-j", "MASQUERADE")
	case "darwin":
		// setup for macOS
		// TODO: fix routing settings
		runCmd("/sbin/ifconfig", ifaceName, tunInputIPAdr+"/30", tunOutputIPAdr, "up")
		runCmd("/usr/sbin/sysctl", "-w", "net.inet.ip.forwarding=1")
		runCmd("/sbin/route", "add", tunInputIPAdr+"/30", tunOutputIPAdr)
		runCmd("/sbin/route", "add", "128.0/1", tunOutputIPAdr)
		runCmd("/sbin/route", "add", "0.0.0.0/1", tunOutputIPAdr)
	}
}

func teardown(ifaceName string) {
	fmt.Println("Teardown interface: ", ifaceName)
	os := runtime.GOOS
	switch os {
	case "linux":
		// teardown for linux
		runCmd("/sbin/iptables", "-t", "nat", "-D", "POSTROUTING", "-o", outgoingNetIface, "-s", tunOutputIPAdr, "-j", "MASQUERADE")
		runCmd("/sbin/ip", "rule", "del", "from", tunOutputIPAdr, "lookup", "main")
		runCmd("/sbin/ip", "rule", "del", "from", "all", "lookup", "Tun")
		runCmd("/sbin/ip", "route", "del", "default", "dev", ifaceName, "table", "Tun")
		runCmd("/sbin/sysctl", "-w", "net.ipv4.ip_forward=0")
		runCmd("/sbin/ifconfig", ifaceName, "down")
	case "darwin":
		// teardown macOS
		runCmd("/sbin/route", "delete", "0.0.0.0/1", tunOutputIPAdr)
		runCmd("/sbin/route", "delete", "128.0/1", tunOutputIPAdr)
		runCmd("/sbin/route", "delete", tunInputIPAdr+"/30", tunOutputIPAdr)
		runCmd("/usr/sbin/sysctl", "-w", "net.inet.ip.forwarding=0")
		runCmd("/sbin/ifconfig", ifaceName, "delete")
	}
}
