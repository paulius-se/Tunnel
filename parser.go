package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"strconv"
	"time"
)

type ruleType int

const (
	notSupported ruleType = 0
	limitTime    ruleType = 1
	limitData    ruleType = 2
)

type rule struct {
	ipAddress   net.IP
	ipNet       *net.IPNet
	ipAddresses []net.IP
	domain      string
	limitValue  int64
	limitUnit   string
	ruleType    ruleType
	limit       int64
	count       int64
	ticker      *time.Ticker
	tickerDone  chan bool
}

func parseLimit(s string) (value int64, unit string) {
	var l, n []rune
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			l = append(l, r)
		case r >= 'a' && r <= 'z':
			l = append(l, r)
		case r >= '0' && r <= '9':
			n = append(n, r)
		}
	}
	val, _ := strconv.ParseInt(string(n), 10, 64)
	return val, string(l)
}

func getBaseValue(value int64, unit string) (mult int64) {
	switch {
	case unit == "kb":
		return 1024 * value
	case unit == "mb":
		return 1024 * 1024 * value
	case unit == "gb":
		return 1024 * 1024 * 1024 * value
	case unit == "s":
		return value
	case unit == "m":
		return 60 * value
	case unit == "h":
		return 60 * 60 * value
	}
	log.Fatalln("Not supported unit specified for base value calculation", unit)
	return 0
}

func getRuleType(unit string) (rt ruleType) {
	switch {
	case unit == "s" || unit == "m" || unit == "h":
		return limitTime
	case unit == "kb" || unit == "mb" || unit == "gb":
		return limitData
	}
	log.Fatalln("Rule contains not supported unit", unit)
	return notSupported
}

func parseIPAddresses(ipAddrsStr []string) (ipAddresses []net.IP) {
	ips := make([]net.IP, 0)
	for _, ipAddrStr := range ipAddrsStr {
		ip := net.ParseIP(ipAddrStr)
		if ip.To4() != nil {
			ips = append(ips, ip)
		}
	}
	return ips
}

func parseRuleSetFile(rulesFile string) (rules []rule) {
	data, err := ioutil.ReadFile(rulesFile)
	checkError(err)
	splitData := bytes.Split(data, []byte("\n"))
	rs := make([]rule, 0)
	for _, line := range splitData {
		splitLineData := bytes.Split(line, []byte(" "))
		if len(splitLineData) > 1 {
			ip, ipnet, parseCIDRError := net.ParseCIDR(string(splitLineData[0]))
			var ipAddressesStr []string
			var domain string
			if parseCIDRError != nil {
				// attempt to resolve domain name
				ipAddrs, lookupHostError := net.LookupHost(string(splitLineData[0]))
				if lookupHostError != nil {
					log.Fatal("The argument", splitLineData[0], "does not appear to be a valid CIDR address nor domain name")
				}
				ipAddressesStr = ipAddrs
				domain = string(splitLineData[0])
			}
			value, unit := parseLimit(string(splitLineData[1]))
			rs = append(rs, rule{
				ipAddress:   ip,
				ipNet:       ipnet,
				ipAddresses: parseIPAddresses(ipAddressesStr),
				domain:      domain,
				limitUnit:   unit,
				limitValue:  value,
				ruleType:    getRuleType(unit),
				limit:       getBaseValue(value, unit),
				count:       0,
			})
		}
	}
	fmt.Println("Parsed rules from", rulesFile)
	for i, rule := range rs {
		if rule.ipNet != nil {
			fmt.Printf("#%v %v %v%v\n", i, rule.ipNet, rule.limitValue, rule.limitUnit)
		} else {
			fmt.Printf("#%v %v %v %v%v\n", i, rule.domain, rule.ipAddresses, rule.limitValue, rule.limitUnit)
		}
	}
	return rs
}
