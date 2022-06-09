package main

import (
	"testing"
	"reflect"
	"strings"
)

func TestUnmarshal(t *testing.T) {
	testcases := []struct{
		name string
		input string
		expected [][]int
	}{
		{
			name: "empty",
			input: `2 2
  
  
`,
			expected: [][]int{{0,0},{0,0}},
		},
		{
			name: "bottom",
			input: `2 2
  
.:
`,
			expected: [][]int{{0,0},{1,2}},
		},
		{
			name: "tables",
			input: `2 2
T:
.T
`,
			expected: [][]int{{-1,2},{1,-1}},
		},
	}
	for _, tc:=range testcases {
		t.Run(tc.name,func(t *testing.T) {
			game := &Board{}
			err := game.Unmarshal(strings.NewReader(tc.input))
			if err!=nil {
				t.Fatalf("unmarshal error: %v",err)
			}
			if !reflect.DeepEqual(game.data,tc.expected) {
				t.Fatalf("expected=%v, got=%v",tc.expected, game.data)
			}
		})
	}
}

func TestRun(t *testing.T) {
	testcases := []struct{
		name string
		input string
		expected string
	}{
		{
			name: "empty",
			input: `2 2
  
  
`,
			expected: `2 2
  
  
`,
		},
		{
			name: "bottom",
			input: `2 2
  
.:
`,
			expected: `2 2
  
.:
`,
		},
		{
			name: "tables",
			input: `3 3
T..
:. 
..T
`,
			expected: `3 3
T  
...
::T
`,
		},
	}
	for _, tc:=range testcases {
		t.Run(tc.name,func(t *testing.T) {
			game:=&Board{}
			if err:=game.Unmarshal(strings.NewReader(tc.input));err!=nil {
				t.Fatalf("unmarshal fail: %v",err)
			}
			game.Run()
			buf := &strings.Builder{}
			game.Marshal(buf)
			if buf.String()!=tc.expected {
				t.Fatalf("expected: %v, got: %v",tc.expected, buf.String())
			}
		})
	}		
}
