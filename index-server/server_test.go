package main
import (
	"testing"
)

func TestIndex(t *testing.T) {
	s,_ := NewServer("")
	if res := s.index("b",[]string{"a"}); res!=FAIL {
		t.Error("index package with missing dependencies shoudl FAIL")
	}
	if res := s.index("a",nil); res!=OK {
		t.Error("index package without dependencies shoudl return OK")
	}
	if res := s.index("b",[]string{"a"}); res!=OK {
		t.Error("index package with all dependencies existing shoudl return OK")
	}
}

func TestQuery(t *testing.T) {
	s,_ := NewServer("")
	if res:=s.query("not_exist"); res!=FAIL {
		t.Error("query not existing package should fail")
	}
	s.index("existing",nil)
	if res:=s.query("existing"); res!=OK {
		t.Error("query existing package should return OK")
	}
}


func TestRemove(t *testing.T) {
	s,_ := NewServer("")
	if res := s.remove("not_exist"); res!=OK {
		t.Error("remove not existing package should return OK")
	}
	s.index("b",nil)
	s.index("a",[]string{"b"})
	if res := s.remove("b"); res!=FAIL {
		t.Error("removing package other depend shoudl return FAIL")
	}
	if res := s.remove("a"); res!=OK {
		t.Error("removing package none depend should return OK")
	}
}
