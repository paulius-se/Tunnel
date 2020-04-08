package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/songgao/water"
	"golang.org/x/net/ipv4"
)

func checksumAccumulate(data []byte, newData bool, accumulator *int32) {
	// Based on ADD_CHECKSUM_32 and SUB_CHECKSUM_32 macros from OpenVPN:
	// https://github.com/OpenVPN/openvpn/blob/58716979640b5d8850b39820f91da616964398cc/src/openvpn/proto.h#L177
	// Assumes length of data is factor of 4.
	for i := 0; i < len(data); i += 4 {
		word := uint32(data[i+0])<<24 | uint32(data[i+1])<<16 | uint32(data[i+2])<<8 | uint32(data[i+3])
		if newData {
			*accumulator -= int32(word & 0xFFFF)
			*accumulator -= int32(word >> 16)
		} else {
			*accumulator += int32(word & 0xFFFF)
			*accumulator += int32(word >> 16)
		}
	}
}

func checksumAdjust(checksumData []byte, accumulator int32) {
	// Based on ADJUST_CHECKSUM macro from OpenVPN:
	// https://github.com/OpenVPN/openvpn/blob/58716979640b5d8850b39820f91da616964398cc/src/openvpn/proto.h#L177
	// Assumes checksumData is 2 byte slice.
	checksum := uint16(checksumData[0])<<8 | uint16(checksumData[1])

	accumulator += int32(checksum)
	if accumulator < 0 {
		accumulator = -accumulator
		accumulator = (accumulator >> 16) + (accumulator & 0xFFFF)
		accumulator += accumulator >> 16
		checksum = uint16(^accumulator)
	} else {
		accumulator = (accumulator >> 16) + (accumulator & 0xFFFF)
		accumulator += accumulator >> 16
		checksum = uint16(accumulator)
	}

	checksumData[0] = byte(checksum >> 8)
	checksumData[1] = byte(checksum & 0xFF)
}

func swapIPAddress(packet []byte, header *ipv4.Header, tunInputIP net.IP, tunOutpuIP net.IP) {
	var sourceIPAddress, destinationIPAddress net.IP
	var checksumIP, checksumTCP, checksumUDP []byte
	checksumIP = packet[10:12]
	sourceIPAddress = packet[12:16]
	destinationIPAddress = packet[16:20]

	var checksumAccumulator int32
	if header.Src.Equal(tunInputIP) {
		checksumAccumulate(sourceIPAddress, false, &checksumAccumulator)
		copy(sourceIPAddress, tunOutpuIP.To4())
		checksumAccumulate(sourceIPAddress, true, &checksumAccumulator)
	} else if header.Dst.Equal(tunOutpuIP) {
		checksumAccumulate(destinationIPAddress, false, &checksumAccumulator)
		copy(destinationIPAddress, tunInputIP.To4())
		checksumAccumulate(destinationIPAddress, true, &checksumAccumulator)
	}

	checksumAdjust(checksumIP, checksumAccumulator)
	if header.Protocol == internetProtocolTCP {
		checksumTCP = packet[36:38]
		checksumAdjust(checksumTCP, checksumAccumulator)
	} else if header.Protocol == internetProtocolUDP {
		checksumUDP = packet[26:28]
		checksumAdjust(checksumUDP, checksumAccumulator)
	}
}

func decodePacket(packetData []byte) (tcpLayerRes *layers.TCP) {
	// Decode a packet
	packet := gopacket.NewPacket(packetData, layers.LayerTypeIPv4, gopacket.Default)
	// Get the TCP layer from this packet
	var tcp *layers.TCP = nil
	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		// Get actual TCP data from this layer
		tcp, _ = tcpLayer.(*layers.TCP)
	}
	return tcp
}

func checkError(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

func writePacket(iface *water.Interface, packet []byte, length int, header *ipv4.Header, tunInputIP net.IP, tunOutpuIP net.IP) {
	swapIPAddress(packet, header, tunInputIP, tunOutpuIP)
	_, e := iface.Write(packet[:length])
	checkError(e)
}

func createTicker(f func()) (t *time.Ticker, c chan bool) {
	done := make(chan bool, 1)
	ticker := time.NewTicker(time.Second * 1)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				f()
			case <-done:
				return
			}
		}
	}()
	return ticker, done
}

func compareIPs(ips []net.IP, ipToCompare net.IP) (eq bool) {
	for _, ip := range ips {
		equal := ipToCompare.Equal(ip)
		if equal {
			return true
		}
	}
	return false
}

func printStatus(rule rule, ruleIdx int, limitReached bool) {
	var displayLimitUnit string

	if limitReached {
		if rule.ruleType == limitTime {
			displayLimitUnit = "sec"
		} else if rule.ruleType == limitData {
			displayLimitUnit = "bytes"
		}
		if rule.ipNet != nil {
			fmt.Printf("\033[2K\r%v limit %v %v reached for rule #%v", rule.ipNet, rule.limit, displayLimitUnit, ruleIdx)
		} else {
			fmt.Printf("\033[2K\r%v limit %v %v reached for rule #%v", rule.domain, rule.limit, displayLimitUnit, ruleIdx)
		}
		return
	}

	if rule.ruleType == limitTime {
		displayLimitUnit = "sec time passed"
	} else if rule.ruleType == limitData {
		displayLimitUnit = "bytes transfered"
	}
	if rule.ipNet != nil {
		fmt.Printf("\033[2K\rwrite packet for %v, %v of %v %v for rule #%v", rule.ipNet, rule.count, rule.limit, displayLimitUnit, ruleIdx)
	} else {
		fmt.Printf("\033[2K\rwrite packet for %v, %v of %v %v for rule #%v", rule.domain, rule.count, rule.limit, displayLimitUnit, ruleIdx)
	}
}
