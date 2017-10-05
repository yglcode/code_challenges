package main

import (
	"strings"
	"log"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
)

const (
	DEF_MYSQL_ACCESS="dev123:1234abcd@/webnmap"
	TIMESTAMP_FORMAT="2006-01-02 15:04:05"
	STR_SEPARATOR = ", "
)

type DBAccess struct {
	dataSrcName string
	db *sql.DB
	stmtInsert *sql.Stmt
	stmtSelect *sql.Stmt
}

func NewDBAccess(dsn ...string) (dba *DBAccess) {
	dba = &DBAccess{}
	dba.dataSrcName = DEF_MYSQL_ACCESS
	if dsn != nil && len(dsn) > 0 {
		dba.dataSrcName = dsn[0]
	}
	return
}

func (dba *DBAccess) Close() {
	dba.db.Close()
	dba.stmtInsert.Close()
	dba.stmtSelect.Close()
}

func (dba *DBAccess) Open() (err error) {
	dba.db, err = sql.Open("mysql", dba.dataSrcName)
	if err != nil {
		log.Println(err.Error())
		return
	}

	// Prepare statement for inserting data
	dba.stmtInsert, err = dba.db.Prepare("INSERT INTO scans (host_ip, scan_time, ports, ports_add, ports_del) VALUES ( ?, ?, ?, ?, ? )")
	if err != nil {
		log.Println(err.Error()) 
		return
	}

	// Prepare statement for reading data
	dba.stmtSelect, err = dba.db.Prepare("SELECT host_ip, scan_time, ports, ports_add, ports_del FROM scans ORDER BY host_ip, scan_time")
	if err != nil {
		log.Println(err.Error())
	}
	return
}

func (dba *DBAccess) SaveScan(host_ip string, scan *Scan) (err error) {
	_, err = dba.stmtInsert.Exec(host_ip, scan.Timestamp, strings.Join(scan.Ports,STR_SEPARATOR), strings.Join(scan.PortsAdd,STR_SEPARATOR), strings.Join(scan.PortsDel,STR_SEPARATOR))
	return
}

func (dba *DBAccess) LoadScanHistories() (data map[string][]*Scan, err error) {
	var host, stime, ports, porta, portd []byte;
	rows, err := dba.stmtSelect.Query()
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer rows.Close()
	data = make(map[string][]*Scan)
	for rows.Next() {
		err = rows.Scan(&host,&stime,&ports,&porta,&portd)
		if err != nil {
			log.Println("row scan err: ",err.Error())
		}
		hostip := string(host)
		data[hostip] = append(data[hostip], &Scan{string(stime),strings.Split(string(ports),STR_SEPARATOR), strings.Split(string(porta),STR_SEPARATOR), strings.Split(string(portd),STR_SEPARATOR)})
	}
	if err = rows.Err(); err != nil {
		log.Println("rows err: ",err.Error())
	}
	return
}

