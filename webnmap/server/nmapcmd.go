package main

import (
	"os/exec"
	"bufio"
	"unicode"
	"bytes"
	"io"
)

var portLine = [][]byte{[]byte("PORT"),[]byte("STATE"),[]byte("SERVICE")}

func isPortTableBegin(line []byte) bool {
	for i:=0;i<len(portLine);i++ {
		if idx := bytes.Index(line, portLine[i]); idx < 0 {
			return false
		} else {
			line = line[idx+len(portLine[i]):]
		}
	}
	return true
}

func decodePortLine(line []byte) (port, status string) {
	p1,p2,p3 := -1,-1,-1
	for i:=0;i<len(line);i++ {
		if unicode.IsSpace(rune(line[i])) {
			if p1 < 0 {
				p1 = i
			} else if p2 > 0 && p3 < 0 {
				p3 = i
				break
			}
		} else {
			if p1 > 0 && p2 < 0 {
				p2 = i
			}
		}
	}
	return string(line[0:p1]), string(line[p2:p3])
}

func decodeNMapOutput(stdout io.Reader) (ports []string) {
	prtTblBgn, prtTblEnd := false, false
	cmdout := bufio.NewReader(stdout)
	line, err1 := cmdout.ReadBytes('\n')
	for err1 == nil && prtTblEnd == false {
		if !prtTblBgn {
			prtTblBgn = isPortTableBegin(line)
		} else if line[0]=='|' {
			//continue line for current port entry, skip
			//fmt.Println("find continueline: ",line)
		} else if unicode.IsDigit(rune(line[0])) {
			//decode one port line, get new open port
			//fmt.Println("find port line: ",line)
			port, status := decodePortLine(line)
			if status == "open" {
				ports = append(ports, port)
			}
		} else {
			//end of port table
			prtTblEnd = true
			continue
		}
		line, err1 = cmdout.ReadBytes('\n')
	}
	return
}

func runNMapCMD(hostip string) (ports []string, err error) {
	cmd := exec.Command("nmap", "-p", "1-1000",hostip)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	if err = cmd.Start(); err != nil {
		return
	}
	//process cmd output, till all port table are read
	ports = decodeNMapOutput(stdout)
	//wait for command to exit
	if err = cmd.Wait(); err != nil {
		return
	}
	return
}

