CREATE USER 'dev123'@'localhost' IDENTIFIED BY '1234abcd';

CREATE DATABASE webnmap;

GRANT ALL ON webnmap.* to 'dev123'@'localhost';

USE webnmap;

CREATE TABLE scans (
       id INT NOT NULL PRIMARY KEY AUTO_INCREMENT,
       host_ip VARCHAR(255),
       scan_time TIMESTAMP,
       ports TEXT,       
       ports_add TEXT,
       ports_del TEXT
);

CREATE INDEX hostip_time ON scans (host_ip,scan_time);

