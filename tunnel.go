package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"

	"github.com/songgao/water"
	"golang.org/x/net/ipv4"
)

const (
	internetProtocolICMP = 1
	internetProtocolTCP  = 6
	internetProtocolUDP  = 17
)

var (
	rulesFile = flag.String("r", "", "path to rule set file")
)

func main() {
	// parse arguments
	flag.Parse()
	if "" == *rulesFile {
		flag.Usage()
		log.Fatalln("\nRule set file is not specified")
	}
	// parse rule set file
	var rules []rule
	rules = parseRuleSetFile(*rulesFile)

	// create and setup tun interface
	iface, err := water.New(water.Config{
		DeviceType: water.TUN,
	})
	if err != nil {
		log.Fatal(err)
	}
	setup(iface.Name())

	// capture ctrl+c and teardown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for sig := range c {
			fmt.Printf("\ncaptured %v, teardown interface and exit\n", sig)
			teardown(iface.Name())
			os.Exit(1)
		}
	}()

	packet := make([]byte, 2000)
	tunInputIP := net.ParseIP(tunInputIPAdr)
	tunOutputIP := net.ParseIP(tunOutputIPAdr)

	fmt.Println("Status:")

	for {
		// Read packet from tun interface
		length, errRead := iface.Read(packet)
		checkError(errRead)
		header, _ := ipv4.ParseHeader(packet[:length])

		// Apply parsed rules from last to first from the rules list
		for i := len(rules) - 1; i >= 0; i-- {
			rule := rules[i]
			if rule.ipNet.Contains(header.Src) || rule.ipNet.Contains(header.Dst) {
				if rule.ruleType == limitData {
					// Apply limit data rules to IP packets
					if rule.count < rule.limit {
						rules[i].count += int64(length)
						fmt.Printf("\033[2K\rwrite packet for %v, %v of %v bytes transfered for rule #%v", rule.ipAddress, rule.count, rule.limit, i)
						writePacket(iface, packet, length, header, tunInputIP, tunOutputIP)
					} else {
						fmt.Printf("\033[2K\r%v limit %v bytes reached for rule #%v", rule.ipAddress, rule.limit, i)
					}
				} else if rule.ruleType == limitTime &&
					(header.Protocol == internetProtocolTCP ||
						header.Protocol == internetProtocolICMP) {
					// Apply limit time rules to TCP packets, pass-through ICMP to enable ping
					tcp := decodePacket(packet[:length])
					if tcp != nil && tcp.SYN && tcp.ACK {
						// start ticker on established connection
						if rules[i].ticker == nil {
							ticker, done := createTicker(func() {
								if rules[i].count < rule.limit {
									rules[i].count++
								} else {
									rules[i].tickerDone <- true
								}
							})
							rules[i].ticker = ticker
							rules[i].tickerDone = done
						}
					}
					if rule.count < rule.limit {
						fmt.Printf("\033[2K\rwrite packet for %v, %v of %v sec time passed for rule #%v", rule.ipAddress, rule.count, rule.limit, i)
						writePacket(iface, packet, length, header, tunInputIP, tunOutputIP)
					} else {
						fmt.Printf("\033[2K\r%v limit %v sec reached for rule #%v", rule.ipAddress, rule.limit, i)
					}
				}
				break
			} else if header.Protocol == internetProtocolUDP {
				// allow UDP anywhere to enable DNS
				writePacket(iface, packet, length, header, tunInputIP, tunOutputIP)
				break
			}
		}
	}
}
