package main

import (
	"bufio"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
)

func TestReadEntry(t *testing.T) {
	fd, err := os.Open("./testdata")
	if err != nil {
		t.Error(err)
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)

	entry, _ := readEntry(scanner)
	if entry != ": 1471766782:0;git status" {
		t.Error(entry)
	}
	entry, _ = readEntry(scanner)
	if entry != ": 1471766797:0;git commit -am \"Update README.md with split command arguments.\"" {
		t.Error(entry)
	}
	entry, _ = readEntry(scanner)
	if entry != ": 1471766804:3;git push origin master" {
		t.Error(entry)
	}
	entry, _ = readEntry(scanner)
	if entry != `: 1472100273:0;echo "hello
world"` {
		t.Error(entry)
	}
	entry, _ = readEntry(scanner)
	if entry != `: 1472100278:0;echo "hello\
world"` {
		t.Error(entry)
	}
	entry, _ = readEntry(scanner)
	if entry != `: 1472100290:0;git commit -m 'rebuild the dam
` {
		t.Error(entry)
	}
	entry, _ = readEntry(scanner)
	if entry != `: 1472100284:0;echo "hello
cruel

world"` {
		t.Error(entry)
	}
}

func TestParseEntry(t *testing.T) {

	//basic entry
	parsed, err := parseEntry(": 1471766782:0;git status")
	if err != nil {
		t.Error(err)
	}
	if parsed.started != "1471766782" {
		t.Error(parsed)
	}
	if parsed.duration != "0" {
		t.Error(parsed)
	}
	if parsed.cmd != "git status" {
		t.Error(parsed)
	}

	//has nonzero duration and semicolon in command
	parsed, err = parseEntry(": 1471766804:3;git commit -am \"Foo\";git push origin master")
	if err != nil {
		t.Error(err)
	}
	if parsed.started != "1471766804" {
		t.Error(parsed)
	}
	if parsed.duration != "3" {
		t.Error(parsed)
	}
	if parsed.cmd != "git commit -am \"Foo\";git push origin master" {
		t.Error(parsed)
	}

	//multiline command
	parsed, err = parseEntry(`: 1472100284:0;echo "hello
cruel

world"`)
	if err != nil {
		t.Error(err)
	}
	if parsed.started != "1472100284" {
		t.Error(parsed)
	}
	if parsed.duration != "0" {
		t.Error(parsed)
	}
	if parsed.cmd != `echo "hello
cruel

world"` {
		t.Error(parsed)
	}

}

func TestInsertEntry(t *testing.T) {
	testDb := filepath.Join(os.TempDir(), "histdb-import-test.db")
	defer os.Remove(testDb)

	db, err := sql.Open("sqlite3", testDb)
	if err != nil {
		t.Error(err)
	}
	defer db.Close()
	_, err = db.Exec(`
	   create table commands (argv text, unique(argv) on conflict ignore);
	   create table places   (host text, dir text, unique(host, dir) on conflict ignore);
	   create table history  (session int,
	                          command_id int references commands (rowid),
	                          place_id int references places (rowid),
	                          exit_status int,
	                          start_time int,
	                          duration int);`)
	if err != nil {
		t.Error(err)
	}

	tx, err := beginTransaction(db)
	if err != nil {
		t.Error(err)
	}

	err = tx.insertEntry(basicEntry{
		started:  "1472100284",
		duration: "3",
		cmd: `echo "hello
cruel

world"`,
	})
	if err != nil {
		t.Error(err)
	}
	if err = tx.Commit(); err != nil {
		t.Error(err)
	}
	rows, err := db.Query("SELECT rowid,argv from commands;")
	if err != nil {
		t.Error(err)
	}
	if !rows.Next() {
		t.Error("No results")
	}
	var cmdId uint64
	var cmd string
	err = rows.Scan(&cmdId, &cmd)
	if err != nil {
		t.Error(err)
	}
	if cmd != `echo "hello
cruel

world"` {
		t.Error(cmd)
	}

	rows, err = db.Query("SELECT rowid,host,dir from places;")
	if err != nil {
		t.Error(err)
	}
	if !rows.Next() {
		t.Error("No results")
	}
	var placeId uint64
	var host, dir string
	err = rows.Scan(&placeId, &host, &dir)
	if err != nil {
		t.Error(err)
	}
	if host != hostName {
		t.Error(host)
	}
	if dir != homeDir {
		t.Error(dir)
	}

	rows, err = db.Query("SELECT session,exit_status,start_time,duration,command_id,place_id from history;")
	if err != nil {
		t.Error(err)
	}
	if !rows.Next() {
		t.Error("No results")
	}
	var historyCmdId, historyPlaceId uint64
	var session_id, exit_status, start_time, duration string
	err = rows.Scan(&session_id, &exit_status, &start_time, &duration, &historyCmdId, &historyPlaceId)
	if err != nil {
		t.Error(err)
	}
	if session_id != sessionNum {
		t.Error(session_id)
	}
	if exit_status != retVal {
		t.Error(exit_status)
	}
	if start_time != "1472100284" {
		t.Error(start_time)
	}
	if duration != "3" {
		t.Error(duration)
	}
	if historyCmdId != cmdId {
		t.Error(historyCmdId)
	}
	if historyPlaceId != placeId {
		t.Error(historyPlaceId)
	}

}
