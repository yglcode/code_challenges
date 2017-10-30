package main
import (
	"log"
	"encoding/json"
	"bytes"
)
//dependencies for each package
type Deps struct {
	parent map[string]bool  //pkgs which depend on this pkg
	children map[string]bool  //pkgs this pkg depends on
}

func (d *Deps) MarshalJSON() ([]byte, error) {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	all := []map[string]bool{d.parent,d.children}
	if err := enc.Encode(all); err != nil { 
		log.Println("Fail to encode Deps")
		return nil,err 
	}
	log.Println("marsh:",buf.String())
	return buf.Bytes(),nil
}
func (d *Deps) UnmarshalJSON(b []byte) error {
	var all []map[string]bool
	if err := json.Unmarshal(b, &all); err != nil { return err }
	d.parent, d.children = all[0],all[1]
	return nil
}
