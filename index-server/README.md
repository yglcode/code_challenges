Index-Server
============

. use RWMutex protect shared index registry

  . index and remove will use WLock and query use RLock

. back up index registry periodically.

. unit test

. add STATUS command to check server status and hook in docker container health checking

. Dockerfile and docker-compose.yml

. build & run:

  docker-compose build

  docker-compose up -d

  docker-compose push

  docker stack deploy -d docker-compose indexer

