# simple web interface to nmap - another interview code project

Given a webpage with a blank text box
When a user types in an ip or hostname
Then an NMAP scan is done (using the NMAP command line tool) against ports 0-1000 which returns the open ports back the UI page for that host

Given a webpage with a blank text box
When a user types in an invalid ip or hostname
Then an error message should appear asking the user to re-submit

Given a user submitting an ip or hostname into the textbox
When ports are different from the previous query
Then a list of ports that have been added and/or subtracted will be visible to the user

# implementation

1. summary
the project is implemented in golang + golang-mysql-driver.
the project runs at a machine with golang dev env, mysql server running and nmap tool installed.

project pkg "webnmap.tar.gz" contains two sub folders:
db: schema.sql script to setup database
server: golang code + html template

2. mysql setup
untar project pkg "webnmap.tar.gz"
cd folder "db"
run schema.sql with admin acct to add user "dev123" and database schema:
mysql -u admin -p < db/schema.sql
verify the following items created:
user "dev123"
database "webnmap"
table "scans"
index hostip_time

3. setup and run server

1> install golang-sql-driver:
go get github.com/go-sql-driver/mysql
2> untar project pkg "webnmap.tar.gz" under $GOPATH/src;
   cd into $GOPATH/src/webnmap/server
   go build
   ./server
   or 
   go run

4. testing from web page:
   
point browser to http://host:8080/scanpage
enter valid ip or hostname to see port scaned
continue to see history works
enter invalid ip or host to see it warned

5. web service api test:

curl http://host:8080/scan/hostname_or_ip, such as 
curl http://host:8080/scan/192.168.0.123
check correct JSON doc returned.

6. unit test
 
cd to webnmap/server
go test
