package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"time"
)

/*
Each character should be one of the following:
  '.' one single rock
  ':' two rocks
  ' ' empty space, into which rocks may fall
  'T' table, through which rocks may not fall
*/

// Game board
type Board struct {
	// store board status in matrix of ints
	// mapping of cell: 'T':-1, ' ':0, '.':1, ':'2 
	data [][]int
}

// Generate a random board layout
func (b *Board) Random(nrow, ncol int) {
	b.data = make([][]int, nrow)
	for i := 0; i < nrow; i++ {
		b.data[i] = make([]int, ncol)
	}
	//set board randomly
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < (nrow * ncol); i++ {
		rc := rand.Intn(nrow * ncol)
		r, c := rc/ncol, rc%ncol
		v := rand.Intn(3)
		b.data[r][c] = v
	}
	//add a few tables
	for i := 0; i < (nrow*ncol)/10; i++ {
		rc := rand.Intn(nrow * ncol)
		r, c := rc/ncol, rc%ncol
		b.data[r][c] = -1
	}
}

// Marshal board layout into "w" io.Writer
func (b Board) Marshal(w io.Writer) {
	if b.data == nil {
		return
	}
	nr, nc := len(b.data), len(b.data[0])
	fmt.Fprintf(w, "%d %d\n", nc, nr)
	line := make([]byte, nc)
	for i := 0; i < nr; i++ {
		for j := 0; j < nc; j++ {
			switch b.data[i][j] {
			case -1:
				line[j] = 'T'
			case 0:
				line[j] = ' '
			case 1:
				line[j] = '.'
			case 2:
				line[j] = ':'
			}
		}
		fmt.Fprintf(w, "%s\n", string(line))
	}
}

// Load and run game from "r" io.Reader
func (b *Board) Unmarshal(r io.Reader) error {
	var nr, nc int
	if _, err := fmt.Fscanf(r, "%d %d\n", &nc, &nr); err != nil {
		return fmt.Errorf("invalid dimensions: %v", err)
	}
	b.data = make([][]int, nr)
	br := bufio.NewReader(r)
	for i := 0; i < nr; i++ {
		line, err := br.ReadBytes('\n')
		if err != nil || len(line) != nc+1 {
			return fmt.Errorf("invalid row data: \"%s\"", line)
		}
		row := make([]int, nc)
		for j := 0; j < nc; j++ {
			switch line[j] {
			case 'T':
				row[j] = -1
			case ' ':
				row[j] = 0
			case '.':
				row[j] = 1
			case ':':
				row[j] = 2
			}
		}
		b.data[i] = row
	}
	return nil
}

// Run game
func (b *Board) Run() {
	nr, nc := len(b.data), len(b.data[0])
	accum := make([]int, nc)
	fillUpto := func(i, j, v int) {
		for v > 0 && i >= 0 {
			if v >= 2 {
				b.data[i][j] = 2
				v -= 2
				i--
			} else {
				b.data[i][j] = 1
				v = 0
			}
		}
	}
	for i := 0; i < nr; i++ {
		for j := 0; j < nc; j++ {
			if v := b.data[i][j]; v < 0 {
				//reach a table
				fillUpto(i-1, j, accum[j])
				accum[j] = 0
			} else { //v>=0
				accum[j] += b.data[i][j]
				b.data[i][j] = 0
			}
		}
	}
	for j := 0; j < nc; j++ {
		fillUpto(nr-1, j, accum[j])
	}
}

// Command line flags for setup
// to generate test data, encode board dimension as Num_Col x Num_Row
var dataGenFlag = flag.String("d", "", "dimensions of game board encoded as COLxROW, such as 7x4")
var dumpInputFlag = flag.Bool("s", true, "dump input")

func main() {
	flag.Parse()
	game := &Board{}
	if len(*dataGenFlag) > 0 {
		var nrow, ncol int
		_, err := fmt.Sscanf(*dataGenFlag, "%dx%d", &ncol, &nrow)
		if err != nil || ncol <= 0 || nrow <= 0 {
			log.Fatalf("invalid game board: %s", *dataGenFlag)
		}
		game.Random(nrow, ncol)
		game.Marshal(os.Stdout)
		return
	}
	if err:=game.Unmarshal(os.Stdin);err!=nil {
		log.Fatalln(err)
	}
	if *dumpInputFlag {
		fmt.Println("--- game input ---")
		game.Marshal(os.Stdout)
		fmt.Println("--- game result ---")
	}
	game.Run()
	game.Marshal(os.Stdout)
}
