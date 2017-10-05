package main

import (
	//"fmt"
	"net"
	"net/http"
	"html/template"
	"strings"
	"regexp"
	"encoding/json"
	"log"
)

type ScanSession struct {
	HostIP string
	Status string
	Scans []*Scan
}

var templates = template.Must(template.ParseFiles("page.html"))

func renderTemplate(w http.ResponseWriter, tmpl string, p *ScanSession) {
	err := templates.ExecuteTemplate(w, tmpl+".html", p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

//use regexp pattern to check hostname
//use golang net.ParseIP() to validate ip address
var hostNameRgx = regexp.MustCompile(`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])(\.[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]*[a-zA-Z0-9])*$`)

func validHostIP(hostip string) bool {
	if len(hostip) == 0 {
		return false
	}
	if net.ParseIP(hostip) != nil { //valid ip
		return true
	} else {
		//fmt.Println("check regx: ", hostip)
		//check if it is valid DNS name
		if len(strings.Replace(hostip,".","",-1))<=255 {
			return hostNameRgx.MatchString(hostip)
		}
	}
	return false
}

var nmapScanner = NewNMapScanner()

func scanpageHandler(w http.ResponseWriter, r *http.Request) {
	hostip := r.FormValue("host_ip")
	log.Println("handle page call for: ", hostip)
	var p *ScanSession
	var err error
	if validHostIP(hostip) {
		p, err = nmapScanner.Scan(hostip)
		if err != nil {
			log.Println("Server internal error: ",err.Error())
			//http.Error(w, err.Error(), http.StatusInternalServerError)
			p = &ScanSession{hostip, "Server internal error", nil}
		}
	} else {
		log.Println("Invalid hostname or ip: ", hostip)
		p = &ScanSession{hostip, "Failure: Invalid Hostname or IP address", nil}
	}
	renderTemplate(w, "page", p)
}

func scanHandler(w http.ResponseWriter, r *http.Request) {
	hostip := r.URL.Path[len("/scan/"):]
	log.Println("handle REST call for: ", hostip)
	var p *ScanSession
	var err error
	if validHostIP(hostip) {
		p, err = nmapScanner.Scan(hostip)
		if err != nil {
			log.Println("Server internal error: ",err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		log.Println("Invalid hostname or ip: ", hostip)
		http.Error(w, "Failure: Invalid Hostname or IP address", http.StatusNotFound)
		return
	}
	err = json.NewEncoder(w).Encode(p)
	if err != nil {
		log.Println("Failed to send JSON data for: ", hostip)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}


func main() {
	nmapScanner.Open()
	defer nmapScanner.Close()
	http.HandleFunc("/scanpage", scanpageHandler)
	http.HandleFunc("/scan/", scanHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
