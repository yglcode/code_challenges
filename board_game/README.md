## Particle Simulation

### how to run

* generate random game board:

    go run main.go -d 8x5 //generate board with 5 rows 8 columns
    
* run game for a random board:

    go run main.go -d 7x4 | go run main.go 

### Requirements:
Your goal is to write a small particle simulation in Go to show how little rocks would fall in a 2D world. Your program should load a model of the simulation state from STDIN. It should then determine how, under the effects of gravity, any loose rocks would fall. Finally, it should write the resulting state to STDOUT.
Input Format
The first line of input will be two integers, separated by a space. These will specify the number of cells in your simulation (width and height, respectively).

After this, each line of input describes one row of cells in your simulation, using the following text representation.

```
Each character should be one of the following:
  '.' one single rock
  ':' two rocks
  ' ' empty space, into which rocks may fall
  'T' table, through which rocks may not fall
```

Tables themselves never fall; they are securely fixed in place. As implied above, rocks may stack up to two into a space.

Example Input and Output:

```
Input:
7 4
.....::
.T    :
 ..T .:
  .   :

Output:
 .    :
 T .  :
  .T .:
:.: .::
```
