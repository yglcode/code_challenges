package main

import (
	"testing"
	"strings"
)

var testData = `
Nmap scan report for scanme.nmap.org (64.13.134.52)
Host is up (0.045s latency).
Not shown: 993 filtered ports
PORT      STATE  SERVICE VERSION
22/tcp    open   ssh     OpenSSH 4.3 (protocol 2.0)
| ssh-hostkey: 1024 60:ac:4d:51:b1:cd:85:09:12:16:92:76:1d:5d:27:6e (DSA)
|_2048 2c:22:75:60:4b:c3:3b:18:a2:97:2c:96:7e:28:dc:dd (RSA)
25/tcp    closed smtp
53/tcp    open   domain
70/tcp    closed gopher
80/tcp    open   http    Apache httpd 2.2.3 ((CentOS))
|_html-title: Go ahead and ScanMe!
| http-methods: Potentially risky methods: TRACE
|_See http://nmap.org/nsedoc/scripts/http-methods.html
113/tcp   closed auth
31337/tcp closed Elite
Device type: general purpose
Running: Linux 2.6.X
OS details: Linux 2.6.13 - 2.6.31, Linux 2.6.18
Network Distance: 13 hops

TRACEROUTE (using port 80/tcp)
HOP RTT       ADDRESS
[Cut first 10 hops for brevity]
11  80.33 ms  layer42.car2.sanjose2.level3.net (4.59.4.78)
12  137.52 ms xe6-2.core1.svk.layer42.net (69.36.239.221)
13  44.15 ms  scanme.nmap.org (64.13.134.52)

Nmap done: 1 IP address (1 host up) scanned in 22.19 seconds
`
func TestDecodeNMapOutput(t *testing.T) {
	expected := []string{"22/tcp", "53/tcp", "80/tcp"}
	result := decodeNMapOutput(strings.NewReader(testData))
	if len(expected) != len(result) {
		t.Errorf("DecodeNMapOut, expected %v, got %v\n", expected, result)
	}
	for i:=0;i<len(expected);i++ {
		if expected[i] != result[i] {
			t.Errorf("DecodeNMapOut, expected %v, got %v\n", expected, result)
			return
		}
	}
}

