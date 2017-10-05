package main

import (
	"fmt"
	"sort"
	"bufio"
	"os"
)

//sort strings by len
type BytesSlice [][]byte
func (ss BytesSlice) Len() int { return len(ss) }
func (ss BytesSlice) Swap(i,j int) { ss[i], ss[j] = ss[j], ss[i] }
func (ss BytesSlice) Less(i,j int) bool { return len(ss[i]) > len(ss[j]) }

func findParts(data []byte, lkup map[string]int, res *[]string) (found bool) {
	if len(*res) > 0 { //avoid find target string itself
		prefix := string(data)
		if lkup[prefix] > 0 {
			*res = append(*res, prefix)
			return true
		}
	}
	l0 := len(data)
	//start from longest subpart to shortest to find parts
	for i:=l0-1;i>0;i-- {
		prefix := string(data[:i])
		if lkup[prefix] > 0 {
			lkup[prefix]--
			*res = append(*res, prefix)
			found = findParts(data[i:], lkup, res)
			if found {
				break
			}
			//backtrack
			lkup[prefix]++
			*res = (*res)[:len(*res)-1]
		}
	}
	return
}

func FindLongestCompound(data [][]byte) (string, []string) {
	//1. sort strings from longest to shortest
	l0 := len(data)
	sort.Sort(BytesSlice(data))
	//fmt.Println(string(data[0]), string(data[len(data)-1]))
	fmt.Println("--- finish sorting ---")
	//2. to speed up search, build hashmap of strings
	lkup := make(map[string]int)
	for i:=0;i<l0;i++ {
		lkup[string(data[i])]++
	}
	fmt.Println("--- finish building lookup table ---")
	//3. start from longest string to shortest string to find 1st compound
	for i:=0;i<l0-1;i++ {
		var res []string
		found := findParts(data[i], lkup, &res)
		if found {
			return string(data[i]), res
		}
	}
	return "",nil
}

func usage () {
	fmt.Println("Usage: cmd input-file-name")
}

func main() {
	if len(os.Args) != 2 {
		usage()
		return
	}
	fname := os.Args[1]
	file, err := os.Open(fname)
	if err != nil {
		fmt.Println("failed to open file: ",fname)
		return
	}
	var data [][]byte
	fscan := bufio.NewScanner(file)
	for fscan.Scan() {
		bs := fscan.Bytes()
		dd := make([]byte, len(bs))
		copy(dd,bs)
		data = append(data, dd)
	}
	if err = fscan.Err(); err != nil {
		fmt.Println("failed to read data: ", fname)
		return
	}
	fmt.Println("read data: ", len(data))
	res, parts := FindLongestCompound(data)
	if parts == nil {
		fmt.Println("failed to find compound")
	} else {
		fmt.Println("found longest compound: ", res, ", and its parts: ", parts)
	}
} 
