package main

import (
	"fmt"
	"sort"
	"bytes"
	"bufio"
	"os"
)

//sort strings by len
type BytesSlice [][]byte
func (ss BytesSlice) Len() int { return len(ss) }
func (ss BytesSlice) Swap(i,j int) { ss[i], ss[j] = ss[j], ss[i] }
func (ss BytesSlice) Less(i,j int) bool { return len(ss[i]) > len(ss[j]) }

func findParts(compo []byte, next int, data [][]byte, allBytes int, compoBcounts map[byte]int, bcounts []map[byte]int, res *[][]byte) (found bool) {
	//fmt.Println("1--",string(compo),compoBcounts,bcounts)
	if allBytes == 0 { //found it
		return true
	}
	//find next subpart candidate
	for i:=next;i<len(data);i++ { 
		if allBytes < len(data[i]) {
			continue
		}
		ok := true
		counts := bcounts[i]
		for k,v := range counts {
			if compoBcounts[k] < v {
				ok = false
				break
			}
		}
		if !ok { 
			continue
		}
		part := data[i]
		head0 := 0
		head := bytes.Index(compo, part)
		for head >= 0 {
			head += head0
			//fmt.Println("found head: ", head)
			//found one candidate, check further
			for k,v := range counts {
				compoBcounts[k] -= v
			}
			allBytes -= len(part)
			for x := 0; x<len(part); x++ {
				compo[x+head] = 0
			}
			*res = append(*res, part)
			found = findParts(compo, i+1, data, allBytes, compoBcounts, bcounts, res)
			//restore compo
			copy(compo[head:head+len(part)], part)
			if found {
				return
			}
			//backtrack
			*res = (*res)[:len(*res)-1]
			for k,v := range counts {
				compoBcounts[k] += v 
			}
			allBytes += len(part)
			head0 = head+1
			head = bytes.Index(compo[head0:], part)
		}
	}
	return false
}

func FindLongestCompound(data [][]byte) (res string, parts[]string) {
	//1. sort strings from longest to shortest
	l0 := len(data)
	sort.Sort(BytesSlice(data))
	//fmt.Println(string(data[0]), string(data[len(data)-1]))
	fmt.Println("--- finish sorting ---")
	//2. to speed up search, count bytes in each string and accumulate it
	bcounts := make([]map[byte]int,l0)
	for i:=0;i<l0;i++ {
		bcounts[i] = make(map[byte]int)
	}
	allcounts := make(map[byte]int)
	for i:=0;i<l0;i++ {
		for _, b := range data[i] {
			bcounts[i][b]++
			allcounts[b]++
		}
	}
	fmt.Println("--- finish building counts table ---")
	//3. start from longest string to shortest string to find 1st compound
	for i:=0;i<l0-1;i++ {
		//check if string "i" has correct byte counts to be compo
		fmt.Println("-- ", string(data[i]))
		ok := true
		for k,v := range bcounts[i] {
			if (allcounts[k]-v) < v {
				ok = false
				break
			}
		}
		var res1 [][]byte
		if ok {
			//if compo candidate, check if we can find its sub parts
			fmt.Println("-- ", string(data[i]), " is candidate, check it")
			found := findParts(data[i],i+1, data, len(data[i]), bcounts[i], bcounts, &res1)
			if found {
				var res2 []string
				for k:=0;k<len(res1);k++ {
					res2 = append(res2, string(res1[k]))
				}
				return string(data[i]), res2
			}
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
	/*
	data1 := [][]byte { []byte("world"),[]byte("we"),[]byte("are"),[]byte("smallest"),[]byte("weare") }
	sort.Sort(BytesSlice(data1))
	for i:=0;i<len(data1);i++ {
		fmt.Println(string(data1[i]))
	}
        */
}
