// Elevator scheduling system emulation:
// two design considerations:
// 1. use goroutines and channels to model system components and their communication:
//    . Components:
//         Elevator,Floor and Scheduler each run in its own goroutine:
//    . Communication:
//       . pickupReqChan: Floors send PickUp requests to Scheduler
//       . schedChan: Scheduler sends scheduling commands to Elevator
//       . statusChan: Elevators report status (currFloor, goalFloor) to Scheduler
// 2. Scheduling algorithm:
//    keep each elevator runing till end and then reverse direction,
//    should have better efficiency
package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"
)

//-------- Settings for elevator simulator -------
const (
	//default number of floors
	DefaultNumberOfFloors = 5
	//default number of elevators
	DefaultNumberOfElevators = 3
	//default duration to run simulation
	DefaultSimulationSeconds = 30
	//time elevator takes to move to next floor
	ElevatorMoveOneFloorTime = time.Second
	//time range for riders arrive
	FloorRiderArrivalInterval = 3 * time.Second
	//time interval to run scheduler
	SchedulerRunInterval = time.Second
)

//-------- Elevator definitions --------

//For best efficiency:
//1. elevator keep a local schedule of goal floors of all passengers
//2. elevator will run each direction till end & finish all then reverse
type Elevator struct {
	id         int
	currFloor  int
	goalFloor  int
	direction  int //+:up,-:down,0:idle
	localSched []bool
	schedChan  chan *Rider          //recv scheduling cmds from Scheduler
	statusChan chan *ElevatorStatus //send status (currFloor,goalFloor) to Scheduler
}

type ElevatorStatus struct {
	id, currFloor, goalFloor int
	schedChan                chan *Rider //used by Scheduler
}

func NewElevator(index int, mfloor int, statusCh chan *ElevatorStatus) (el *Elevator) {
	el = &Elevator{
		index, 0, 0, 0,
		make([]bool, mfloor),
		make(chan *Rider, mfloor),
		statusCh,
	}
	return
}

//elevators' goroutine Run():
//1. consume scheduler commands from schedulers
//2. update its position by 1 floor
//3. do local schedule, pick up next goto floor
//4. recv exit cmd from Scheduler, cleanup & exit
func (el *Elevator) Run() {
	log.Printf("elevator[%d] starts\n", el.id)
ElevatorLoop:
	for {
		//consume all scheduler & update local sched table
	schedcmd:
		for {
			select {
			case r := <-el.schedChan:
				if r.pickupFloor == -1 && r.goalFloor == -1 {
					break ElevatorLoop
				}
				log.Printf("elevator[%d] recv req %v\n", el.id, r)
				el.localSched[r.pickupFloor] = true
				el.localSched[r.goalFloor] = true
			default:
				if el.WillMove() {
					break schedcmd
				} else {
					//no schedule, will not move, block wait schedule
					r := <-el.schedChan
					if r.pickupFloor == -1 && r.goalFloor == -1 {
						break ElevatorLoop
					}
					log.Printf("elevator[%d] recv req %v\n", el.id, r)
					el.localSched[r.pickupFloor] = true
					el.localSched[r.goalFloor] = true
					continue schedcmd
				}
			}
		}
		//do local schdule, possible update goalFloor
		el.schedule()
		//update its location
		switch {
		case el.direction > 0:
			el.GoUpOneFloor()
		case el.direction < 0:
			el.GoDownOneFloor()
		}
		//if reach goal, find new goal floor
		if el.currFloor == el.goalFloor {
			//reach goal
			el.localSched[el.goalFloor] = false
			el.schedule()
		}
		//report status change to scheduler
		el.statusChan <- &ElevatorStatus{el.id, el.currFloor, el.goalFloor, nil}
	}
	//tell scheduler i exits
	el.statusChan <- &ElevatorStatus{el.id, -1, -1, nil}
	log.Printf("elevator[%d] exits\n", el.id)
}

//is elevator currently moving or scheduled to move
func (el *Elevator) WillMove() bool {
	if el.currFloor != el.goalFloor { //is moving
		return true
	}
	for _, v := range el.localSched {
		if v { //has scheduled
			return true
		}
	}
	return false
}

func (el *Elevator) GoUpOneFloor() {
	//simulate moving distance by 1 sec
	time.Sleep(ElevatorMoveOneFloorTime)
	el.currFloor++
}

func (el *Elevator) GoDownOneFloor() {
	//simulate moving distance by 1 sec
	time.Sleep(ElevatorMoveOneFloorTime)
	el.currFloor--
}

//elevator local scheduling based on local table
func (el *Elevator) schedule() {
	upNext := -1
	for i := el.currFloor + 1; i < len(el.localSched); i++ {
		if el.localSched[i] {
			upNext = i
			break
		}
	}
	downNext := -1
	for i := el.currFloor - 1; i >= 0; i-- {
		if el.localSched[i] {
			downNext = i
			break
		}
	}
	switch {
	case el.direction > 0:
		//currently going up
		switch {
		case upNext >= 0:
			el.goalFloor = upNext
		case downNext >= 0:
			el.goalFloor = downNext
			el.direction = -1 //change direction
		default:
			el.direction = 0
		}
	case el.direction < 0:
		//currently going down
		switch {
		case downNext >= 0:
			el.goalFloor = downNext
		case upNext >= 0:
			el.goalFloor = upNext
			el.direction = 1 //change direction
		default:
			el.direction = 0
		}
	default:
		//currently idle
		switch {
		case upNext >= 0:
			el.goalFloor = upNext
			el.direction = 1
		case downNext >= 0:
			el.goalFloor = downNext
			el.direction = -1
		default:
			el.direction = 0
		}
	}
}

//---------- Floor related definitions --------------

//elevator rider
type Rider struct {
	pickupFloor int
	goalFloor   int
}

//Floor:
// generate random pickup requests for scheduler
// each req is a rider obj
type Floor struct {
	id            int         //floor number
	numFloor      int         //number floors at building
	exitChan      chan bool   //Scheduler send "exit" cmd on this chan
	pickupReqChan chan *Rider //send pickup req to scheduler
}

func NewFloor(index, numF int, pickChan chan *Rider) *Floor {
	return &Floor{
		index, numF,
		make(chan bool, 1),
		pickChan}
}

//Floor goroutine Run() func;
//  1. sleep random interval (0-3 secs)
//  2. generate random pickup req
func (f *Floor) Run() {
	log.Printf("floor[%d] starts\n", f.id)
FloorLoop:
	for {
		select {
		case <-f.exitChan:
			break FloorLoop
		case <-time.After(time.Duration(rand.Float64() * (float64)(FloorRiderArrivalInterval))):
		}
		//let floor generate random requests
		goalFloor := rand.Intn(f.numFloor)
		if f.id != goalFloor {
			f.pickupReqChan <- &Rider{f.id, goalFloor}
			log.Printf("pickup req: floor[%d], dst[%d]\n", f.id, goalFloor)
		} else {
			log.Printf("No pickup req: floor[%d]\n", f.id)
		}
	}
	//tell scheduler i exits
	f.exitChan <- true
	log.Printf("floor[%d] exits\n", f.id)
}

//------------- Scheduler related definitions ------------------

//Elevator scheduler system:
//1. recv pickup reqs from floors
//2. schedule pickups to elevators
//3. recv status updates from elevators
//4. keep all channels communicating with elevators, floors
//5. report scheduler overall status
type Scheduler struct {
	elevStatus     []*ElevatorStatus    //elevator status
	floorExitChan  []chan bool          //chans for send "exit" cmds to floors
	pickupReqChan  chan *Rider          //chan for recv pickup reqs from floors
	elevStatusChan chan *ElevatorStatus //chans for recv elevator status updates
	waitUpQue      [][]*Rider           //queue of riders waiting going up
	waitDownQue    [][]*Rider           //queue of riders waiting going down
	schedTickChan  <-chan time.Time     //time ticks to invoke scheduling
	exitChan       chan bool            //recv "exit" cmd when simulation time is finished
}

func NewScheduler(nfloor, nelevator int, exitCh chan bool) (s *Scheduler) {
	s = &Scheduler{
		make([]*ElevatorStatus, nelevator),
		make([]chan bool, nfloor),
		make(chan *Rider, nfloor),
		make(chan *ElevatorStatus, nelevator),
		make([][]*Rider, nfloor),
		make([][]*Rider, nfloor),
		nil, exitCh,
	}
	//init random number generator
	rand.Seed(time.Now().UnixNano())
	//init status report ticker chan
	s.schedTickChan = time.Tick(SchedulerRunInterval)
	//init & start elevators
	for i := 0; i < nelevator; i++ {
		el := NewElevator(i, nfloor, s.elevStatusChan)
		s.elevStatus[i] = &ElevatorStatus{i, el.currFloor, el.goalFloor, el.schedChan}
		go el.Run()
	}
	//init & start floors
	for i := 0; i < nfloor; i++ {
		f := NewFloor(i, nfloor, s.pickupReqChan)
		s.floorExitChan[i] = f.exitChan
		go f.Run()
	}
	return
}

//get elevator status report
func (s *Scheduler) Status() (res [][]int) {
	for i := 0; i < len(s.elevStatus); i++ {
		res = append(res, []int{s.elevStatus[i].id, s.elevStatus[i].currFloor,
			s.elevStatus[i].goalFloor})
	}
	return
}

/* replaced by Elevator.schedChan
func (s *Scheduler) Update(eid int, cfloor, gfloor int) {
}
*/

/* replaced by Scheduler.pickupReqChan
func (s *Scheduler) Pickup(pickfloor, direction int) {
}
*/

//get string representation of waiting que
func que2Str(wq [][]*Rider) string {
	var res []byte
	for _, v := range wq {
		res = append(res, []byte("[ ")...)
		for _, r := range v {
			res = append(res, []byte(fmt.Sprintf(" %v ", r))...)
		}
		res = append(res, []byte(" ]")...)
	}
	return string(res)
}

// Elevator simulator goroutine Run() func:
// 1. wait for pickup reqs from floors and status updates from elevators
// 2. from pickupReqChan, get all pickup reqs and schedule them to elevators
// 3. print out global status
// 4. orchestrate graceful shutdown of all goroutines
func (s *Scheduler) Run() {
	log.Println("Elevator scheduler starts")
	reportCount := 1
	log.Printf("------------------ Period %d -----------------\n", reportCount)
SchedulerLoop:
	for {
		select {
		case <-s.exitChan:
			break SchedulerLoop //finish
		case <-s.schedTickChan:
			//print status before schedule
			log.Printf("elevators: %v\n", s.Status())
			log.Printf("waitUpQue: %v\n", que2Str(s.waitUpQue))
			log.Printf("waitDownQue: %v\n", que2Str(s.waitDownQue))
			//time to schedule
			s.schedule()
			//record next scheduling period
			reportCount++
			log.Printf("------------------ Period %d -----------------\n", reportCount)
		case st := <-s.elevStatusChan:
			//update elevator status
			//log.Println("Scheduler recv elevator status update: ", st)
			s.elevStatus[st.id].currFloor = st.currFloor
			s.elevStatus[st.id].goalFloor = st.goalFloor
		case r := <-s.pickupReqChan:
			//recv pickup reqs from floors
			//log.Printf("Scheduler recv pickup req %v\n", r)
			if (r.goalFloor - r.pickupFloor) > 0 {
				s.waitUpQue[r.pickupFloor] = append(s.waitUpQue[r.pickupFloor], r)
			} else {
				s.waitDownQue[r.pickupFloor] = append(s.waitDownQue[r.pickupFloor], r)
			}
		}
	}
	//initiate exit process
	log.Println("Scheduler initiate exit process...")
	//1. tell all elevator goroutines to exit
	for _, el := range s.elevStatus {
		el.schedChan <- &Rider{-1, -1}
	}
	//2. tell all floors to exit
	for _, ech := range s.floorExitChan {
		ech <- true
	}
	//3. wait for all elevators and floors to exit
	exitElev := 0
	for es := range s.elevStatusChan {
		if es.currFloor == -1 && es.goalFloor == -1 {
			exitElev++
			if exitElev >= len(s.elevStatus) {
				break
			}
		}
	}
	for _, ech := range s.floorExitChan {
		<-ech
	}
	s.exitChan <- true
	log.Println("Elevator scheduler exits")
}

//for better efficiency, schedule this way:
//1. keep elevators moving in one direction till end then reverse
//2. if there are riders waiting to go up, check if any up-going elevator take them
//3. if there are riders waiting to go down, check if any down-going elevator take them
//4. then check if any idle elevators can take them
//5. otherwise, riders wait in queue till idle elevators go to proper position to take them
func (s *Scheduler) schedule() {
	numWaitUp := 0
	numWaitDown := 0
	for _, v := range s.waitUpQue {
		numWaitUp += len(v)
	}
	for _, v := range s.waitDownQue {
		numWaitDown += len(v)
	}
	if numWaitUp == 0 && numWaitDown == 0 {
		log.Println("no schedule req")
		return
	}
	var upElevs []*ElevatorStatus
	var downElevs []*ElevatorStatus
	var idleElevs []*ElevatorStatus
	for i := 0; i < len(s.elevStatus); i++ {
		switch {
		case s.elevStatus[i].goalFloor > s.elevStatus[i].currFloor:
			upElevs = append(upElevs, s.elevStatus[i])
		case s.elevStatus[i].goalFloor < s.elevStatus[i].currFloor:
			downElevs = append(downElevs, s.elevStatus[i])
		default:
			idleElevs = append(idleElevs, s.elevStatus[i])
		}
	}
	//log.Println("-0",  numWaitUp, numWaitDown, len(upElevs),len(downElevs),len(idleElevs))
	if numWaitUp > 0 {
		for _, el := range upElevs {
			for i := el.currFloor; i < len(s.floorExitChan) && numWaitUp > 0; i++ {
				for j := 0; j < len(s.waitUpQue[i]); j++ {
					el.schedChan <- s.waitUpQue[i][j]
					numWaitUp--
				}
				s.waitUpQue[i] = nil
			}
		}
		//log.Println("-1", numWaitUp, numWaitDown, len(upElevs),len(downElevs),len(idleElevs))
		if numWaitUp > 0 {
			for _, el := range idleElevs {
				for i := el.currFloor; i < len(s.floorExitChan) && numWaitUp > 0; i++ {
					for j := 0; j < len(s.waitUpQue[i]); j++ {
						el.schedChan <- s.waitUpQue[i][j]
						numWaitUp--
					}
					s.waitUpQue[i] = nil
				}
			}
		}
	}
	if numWaitDown > 0 {
		for _, el := range downElevs {
			for i := el.currFloor; i >= 0 && numWaitDown > 0; i-- {
				for j := 0; j < len(s.waitDownQue[i]); j++ {
					el.schedChan <- s.waitDownQue[i][j]
					numWaitDown--
				}
				s.waitDownQue[i] = nil
			}
		}
		if numWaitDown > 0 {
			for _, el := range idleElevs {
				for i := el.currFloor; i >= 0 && numWaitDown > 0; i-- {
					for j := 0; j < len(s.waitDownQue[i]); j++ {
						el.schedChan <- s.waitDownQue[i][j]
						numWaitDown--
					}
					s.waitDownQue[i] = nil
				}
			}
		}
	}
	if len(idleElevs) > 0 {
		pos := 0
		if numWaitUp > 0 {
			jumpBot := 0
			for i := 0; i < len(s.floorExitChan); i++ {
				if len(s.waitUpQue[i]) > 0 {
					jumpBot = i
					break
				}
			}
			idleElevs[pos].schedChan <- &Rider{jumpBot, jumpBot}
			pos++
		}
		if len(idleElevs) > 1 && numWaitDown > 0 {
			jumpTop := 0
			for i := len(s.floorExitChan) - 1; i >= 0; i-- {
				if len(s.waitDownQue[i]) > 0 {
					jumpTop = i
					break
				}
			}
			idleElevs[pos].schedChan <- &Rider{jumpTop, jumpTop}
			pos++
		}

	}
}

//simulation driver:
//1. get simulation parameters from command line or use default
//2. run simulation
//3. when timeout, ask simulation shutdown gracefully
func main() {
	simuSeconds := DefaultSimulationSeconds
	numFloors := DefaultNumberOfFloors
	numElevators := DefaultNumberOfElevators
	//parse input parameters
	var err error
	if len(os.Args) > 1 {
		numFloors, err = strconv.Atoi(os.Args[1])
		if err != nil {
			usage()
			log.Fatal("Invalid input parameters")
		}
	}
	if len(os.Args) > 2 {
		numElevators, err = strconv.Atoi(os.Args[2])
		if err != nil {
			usage()
			log.Fatal("Invalid input parameters")
		}
	}
	if len(os.Args) > 3 {
		simuSeconds, err = strconv.Atoi(os.Args[3])
		if err != nil {
			usage()
			log.Fatal("Invalid input parameters")
		}
	}
	log.Printf("===== Simulation will run %d seconds ======\n", simuSeconds)
	//start elevator simulation
	exitChan := make(chan bool)
	sched := NewScheduler(numFloors, numElevators, exitChan)
	go sched.Run()
	//let simulation run for a while
	time.Sleep(time.Duration(simuSeconds) * time.Second)
	//shutdown
	exitChan <- true
	<-exitChan
}

func usage() {
	fmt.Println("Usage: elevator numFloors numElevators numSimulationSeconds")
}

