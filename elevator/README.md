Elevator scheduling system emulation
====================================

two design considerations.

1. use goroutines and channels to model system components and their communication.

   * Components:
   
      Elevator,Floor and Scheduler each run in its own goroutine.

      * Elevators:
		* receive scheduling commands from Scheduler
		* maintain local schedule table
		* move elevator-car up or down one floor (assuming 1 sec)
		* send status update messages to Scheduler

      * Floors:
		* simulate riders arrival in random interval (0-3 secs)
		* simulate random destination floors
		* send PickUp requests to Scheduler

      * Scheduler:
		* receive PickUp requests from Scheduler
		* do scheduling once per second
		* send scheduling commands to elevators
		* receive status updates from elevators
        
   * Communication:
   
      * pickupReqChan: Floors send PickUp requests to Scheduler
      * schedChan: Scheduler sends scheduling commands to Elevator
      * statusChan: Elevators report status (currFloor, goalFloor) to Scheduler
      
2. Scheduling algorithm.

   keep each elevator runing till end and then reverse direction,
   should have better efficiency
