package main

import (
	"time"
	"sync"
)

type Scan struct {
	Timestamp string
	Ports []string
	PortsAdd []string
	PortsDel []string
}

//simple nmap scanner keeping scan history
type NMapScanner struct {
	data map[string][]*Scan
	sync.Mutex //protect data-map against concurrent access
	dba *DBAccess
}

func NewNMapScanner() (nmsc *NMapScanner) {
	nmsc = &NMapScanner{}
	nmsc.dba = NewDBAccess()
	return
}

func (nmsc *NMapScanner) Open() (err error) {
	err = nmsc.dba.Open()
	if err != nil {
		return
	}
	nmsc.data, err = nmsc.dba.LoadScanHistories()
	return
}

func (nmsc *NMapScanner) Close()  {
	nmsc.dba.Close()
}

//modify data-map under lock protection
func (nmsc *NMapScanner) AddScan(hostip string, sc *Scan) []*Scan {
	nmsc.Lock()
	defer nmsc.Unlock()
	scanHist := append(nmsc.data[hostip], sc)
	nmsc.data[hostip] = scanHist
	calcHist(scanHist)
	return scanHist
}

//calc what ports are added or removed from last scan
func calcHist(sh []*Scan) {
	sz := len(sh)
	if sz <=1 {
		return
	}
	curr,prev := sz-1,sz-2
	currPorts, prevPorts := make(map[string]bool), make(map[string]bool)
	for i:=0;i<len(sh[curr].Ports);i++ {
		currPorts[sh[curr].Ports[i]] = true
	}
	for i:=0;i<len(sh[prev].Ports);i++ {
		prevPorts[sh[prev].Ports[i]] = true
	}
	for i:=0;i<len(sh[curr].Ports);i++ {
		if prevPorts[sh[curr].Ports[i]] == false {
			sh[curr].PortsAdd = append(sh[curr].PortsAdd, sh[curr].Ports[i]) 
		}
	}
	for i:=0;i<len(sh[prev].Ports);i++ {
		if currPorts[sh[prev].Ports[i]] == false {
			sh[curr].PortsDel = append(sh[curr].PortsDel, sh[prev].Ports[i]) 
		}
	}
}

//do nmap scan and update history
func (nmsc *NMapScanner) Scan(hostip string) (p *ScanSession, err error) {
	ports, err := runNMapCMD(hostip)
	if err != nil {
		p = &ScanSession{hostip, "Failure: "+err.Error(), nil}
		return
	}
	sc := &Scan{Timestamp: time.Now().Format(TIMESTAMP_FORMAT), Ports: ports}
	scanHist := nmsc.AddScan(hostip, sc)
	err = nmsc.dba.SaveScan(hostip, sc)
	if err != nil {
		p = &ScanSession{hostip, "Failure: "+err.Error(), nil}
		return
	}
	p = &ScanSession{hostip, "Success", scanHist}
	return
}
