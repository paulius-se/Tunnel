# Tunnel

Simple network filter for the Internet.

The filter is implemented purely in GO (version 1.14 linux/amd64) and based on Tun interface.
Currently supports Linux OS (tested on Ubuntu 18.04.1). Support for Mac OS X and Windows is also possible, but Tun interface and routing setups are different for each OS and not implemented yet.

Rules are specified in a text file lines in the following format:
```
<addresses to limit in CIDR> <limit value><limit unit>
```
e.g.
```
0.0.0.0/0 10s           - allow TCP connection everywhere for 10 seconds
94.142.241.111/32 1mb   - allow download of 1 megabyte using IP packets from 94.142.241.111
```
Supported limit units:

kb, mb, gb - number kilobytes, megabytes or gigabytes computer is allowed to download from an addresses using IP packets.
s, m, h - number of seconds, minutes or hours computer is allowed to access specified addresses using TCP protocol.

Lower rule in the list overwrites previous one for same address.


# Build

In order to build, run the following commands in terminal:

1. install dependecies
```
    $ go get ./...
```    
2. make a build
```
    $ go build .
```
The output file should appear as `tunnel`.


# Prerequisities

Add the routing table entry to /etc/iproute2/rt_tables: 

`200 Tun`

$ cat /etc/iproute2/rt_tables
```
#
# reserved values
#
255	local
254	main
253	default
0	unspec
#
# local
#
#1	inr.ruhep

200	Tun
```

# Run

To run the network filter:
```
$ sudo ./tunnel -r rules.txt
```
Note: 'sudo' is required to make Tun interface and routing setup.


# Usage

Provided rule set file rules.txt contains the following IP addresses in CIDR notation
```
0.0.0.0/0 30s           - allows access to any IP address for 30 seconds
172.217.21.132/32 2m    - allows access to www.google.com (172.217.21.132) for 2 minutes
80.249.99.148/32 11mb   - allows to download 11 megabytes of data from ipv4.download.
thinkbroadband.com (80.249.99.148)
www.youtube.com 1h      - allows youtube for one hour
```
Run the following commands:

1.
```
$ curl http://ipv4.download.thinkbroadband.com/5MB.zip --output 5mb.zip
```
This will activate the rule '80.249.99.148/32 11mb'. Tunnel CLI will output to console the following:
```
Parsed rules from rules.txt
#0 0.0.0.0/0 10s
#1 172.217.21.132/32 2m
#2 80.249.99.148/32 11mb
Setup interface:  tun0
Status:
write packet for 80.249.99.148, 5484083 of 11534336 bytes transfered for rule #2
```
When data transfer limit reached the CLI blocks the IP address and output to console:
```
Status:
80.249.99.148 limit 11534336 bytes reached for rule #2
```

2.
```
$ curl 172.217.21.132
```
The rule '172.217.21.132/32 2m' will be activated. The following CLI output can be observed:
```
Parsed rules from rules.txt
#0 0.0.0.0/0 30s
#1 172.217.21.132/32 2m
#2 80.249.99.148/32 11mb
Setup interface:  tun0
Status:
write packet for 172.217.21.132, 6 of 120 sec time passed for rule #1
```
When specified time ellapsed tunnel CLI blocks access to the IP address and will output to console:
```
Status:
172.217.21.132 limit 120 sec reached for rule #1
```

Press `ctrl+c` to exit tunnel and restore interface and routing settings.
