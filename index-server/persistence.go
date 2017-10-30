package main
import (
	"log"
	"io/ioutil"
	"time"
	"sort"
	"path/filepath"
	"encoding/json"
	"os"
)

func (s *Server) restoreRegistry() {
	if len(s.backupDir)==0 { return }
	files, err := ioutil.ReadDir(s.backupDir)
	if err != nil {
		log.Println("Failed to open backup dir:", s.backupDir)
		return
	}
	//sort files for newest first
	sort.Slice(files,func(i,j int)bool{return files[i].ModTime().After(files[j].ModTime())})
	for i:=0;i<len(files);i++ {
		f, err := os.Open(filepath.Join(s.backupDir,files[i].Name()))
		if err != nil {
			log.Println("Failed open",files[i].Name(),"to restore registry")
			continue
		}
		//s.Lock() - no need lock here, since we haven't start yet
		err = json.NewDecoder(f).Decode(&s.registry)
		//s.Unlock()
		f.Close()
		if err != nil {
			log.Println("Failed to decode file",files[i].Name())
		} else {
			log.Println("Succeed reloading registry file:",files[i].Name())
			break
		}
	}
}

func (s *Server) backupRegistry() {
	if len(s.backupDir)==0 { return }
	s.Lock()
	data,err := json.Marshal(s.registry)
	s.Unlock()
	if err != nil {
		log.Println("Fail to json.Marshal registry:",err)
		return
	}
	//log.Println("debug:",string(data))
	fname := time.Now().Format("Jan-2-2006-15-04-05")+".json"
	fname = filepath.Join(s.backupDir,fname)
	f, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println("Failed to open registry file:",fname)
		return
	}
	if _, err = f.Write(data); err != nil {
		log.Println("Failed to backup registry to file:",fname,"error:",err)
		f.Close()
		return
	}
	if err := f.Close(); err != nil {
		log.Println("Failed to close file",err)
		return
	}

	log.Println("Succeed backing up registry to file:",fname)

	files, err := ioutil.ReadDir(s.backupDir)
	if err != nil {
		log.Println("Failed to open backup dir:", s.backupDir)
		return
	}
	//sort files for newest first
	sort.Slice(files,func(i,j int)bool{return files[i].ModTime().After(files[j].ModTime())})
	if len(files)>2 { //keep max 2 files
		fname = files[len(files)-1].Name()
		err = os.Remove(filepath.Join(s.backupDir,fname))
		if err != nil {
			log.Println("Failed to cleanup backup file:",fname,"errr:",err)
		} else {
			log.Println("Succeed removing old backup file:",fname)
		}
	}
}
