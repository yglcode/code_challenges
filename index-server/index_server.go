package main
import (
	"log"
	"os"
	"sync"
	"sync/atomic"
	"strconv"
	"bufio"
	"strings"
	"io"
	"net"
	"path/filepath"
)
//
const (
	//request commands
	INDEX = "INDEX"
	QUERY = "QUERY"
	REMOVE = "REMOVE"
	STATUS = "STATUS" //added to check server status
	//responses
	OK = "OK"
	FAIL = "FAIL"
	ERROR = "ERROR"
)

type Server struct {
	registry map[string]*Deps
	backupDir string
	sync.RWMutex
	numOK,numERROR,numFAIL int64
}

func NewServer(fname string) (s *Server,err error) {
	if len(fname)>0 && !filepath.IsAbs(fname) {
		var cwd string
		cwd,err = os.Getwd()
		if err != nil { return }
		fname = filepath.Join(cwd,fname)
	}
	s = &Server{registry:make(map[string]*Deps),backupDir:fname}
	if len(fname)>0 {
		err = os.MkdirAll(fname,0755)
		if err != nil { return }
		s.restoreRegistry()
	}
	return
}

func (s *Server) HandleConn(conn net.Conn, stopChan chan struct{}) {
	defer conn.Close()
	//read client request
	for {
		select {
		case _,chanOpen := <- stopChan:
			if !chanOpen { //stopChan closed, exit
				log.Println("Notify handler exit")
				return
			}
		default:
		}
		req, err := bufio.NewReader(conn).ReadString('\n')
		if err != nil {
			log.Println("Failed to read request: ",err)
			if  err != io.EOF {
				atomic.AddInt64(&s.numERROR,1)
				if _,err = conn.Write([]byte(ERROR+"\n")); err != nil {
					log.Println("Fail to send response to client:",err)
				}
			}
			log.Println("done1---")
			return
		}
		req=req[:len(req)-1] //remove delimiter
		//parse & validate req
		if cmd, pkg, deps, ok := parseReq(req); !ok {
			log.Println("Invalid message: ",req)
			atomic.AddInt64(&s.numERROR,1)
			if _,err = conn.Write([]byte(ERROR+"\n")); err != nil {
				log.Println("Fail to send response to client:",err)
			}
		} else {
			//process req
			var rsp string
			switch cmd {
			case INDEX:
				rsp = s.index(pkg,deps)
			case REMOVE:
				rsp = s.remove(pkg)
			case QUERY:
				rsp = s.query(pkg)
			case STATUS:
				data := strconv.FormatInt(atomic.LoadInt64(&s.numOK),10)
				data += ", "+strconv.FormatInt(atomic.LoadInt64(&s.numFAIL),10)
				data += ", "+strconv.FormatInt(atomic.LoadInt64(&s.numERROR),10)
				if _,err = conn.Write([]byte(data+"\n")); err != nil {
					log.Println("Fail to reply status to client:",data,err)
				}
				return
			default:
				rsp = ERROR
			}
			switch rsp {
			case OK: atomic.AddInt64(&s.numOK,1)
			case FAIL: atomic.AddInt64(&s.numFAIL,1)
			case ERROR: atomic.AddInt64(&s.numERROR,1)
			}
			if _,err = conn.Write([]byte(rsp+"\n")); err != nil {
				log.Println("Fail to send response to client:",err)
			}
			log.Println(rsp,": ",req)
		}
	}
}

func parseReq(req string) (cmd, pkg string, deps []string, ok bool) {
	parts := strings.Split(req,"|")
	if len(parts)!=3 { return }
	cmd,pkg = parts[0],parts[1]
	if len(cmd)==0 || (cmd!=STATUS && len(pkg) == 0) {
		return
	}
	ok = true
	if len(parts)==3 && len(parts[2])>0 {
		deps = strings.Split(parts[2],",")
	}
	return
}

func (s *Server) index(pkg string, depNames []string) string {
	log.Println("Index: ",pkg,depNames)
	s.Lock()
	defer s.Unlock()
	//check if already indexed
	deps := s.registry[pkg]
	//check dependencies
	newChildren := make(map[string]bool)
	for _,d := range depNames {
		if dep := s.registry[d]; dep == nil { //miss one dependency
			log.Println("Index failure: missing dependencies [",d,"]")
			if deps != nil { //dependency changed 
				//remove it 
				s.removePkgAndParent(pkg)
			}
			return FAIL
		} else {
			newChildren[d] = true
		}
	}
	//all dependencies exist
	if deps != nil {
		//remove expired dependency
		var changed bool
		for cn, _ := range deps.children {
			if !newChildren[cn] {
				c := s.registry[cn]
				delete(c.parent,pkg)
				changed = true
			}
		}
		if !changed { return OK }
	} else {
		deps = &Deps{parent:make(map[string]bool)}
		log.Println("Add index for: ",pkg)
		s.registry[pkg] = deps
	}
	//replace new dependency
	for cn,_ := range newChildren {
		c := s.registry[cn]
		c.parent[pkg] = true
	}
	deps.children = newChildren
	return OK
}

//remove a pkg if no other pkgs depend on it
func (s *Server) remove(pkg string) string {
	s.Lock()
	defer s.Unlock()
	deps := s.registry[pkg]
	if deps != nil {
		if len(deps.parent)>0 { 
			return FAIL 
		} else {
			//remove pkg
			delete(s.registry,pkg)
			//clean its dependencies parent
			for cn,_ := range deps.children {
				c := s.registry[cn]
				delete(c.parent,pkg)
			}
		}
	}
	return OK
}

//remove pkg and its parent recursively
func (s *Server) removePkgAndParent(pkg string) {
	deps := s.registry[pkg]
	if deps != nil {
		delete(s.registry,pkg)
		log.Println("remove ",pkg," because dependencies lost")
		//clean its dependencies parent
		for cn,_ := range deps.children {
			c := s.registry[cn]
			delete(c.parent,pkg)
		}
		//remove its parents, since miss dependency
		for pn,_ := range deps.parent {
			s.removePkgAndParent(pn)
		}
	}
}

//query if pkg is indexed or not
func (s *Server) query(pkg string) string {
	s.RLock()
	defer s.RUnlock()
	if s.registry[pkg] == nil {
		return FAIL
	} 
	return OK
}
