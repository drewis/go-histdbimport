/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */
package main

import (
	"bufio"
	"database/sql"
	"errors"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

//used for dir column
var homeDir = os.Getenv("HOME")

//used for host column
var hostName string

//used for session column
var sessionNum = "0"

//used for exit_status column
var retVal = "0"

//representation of a history entry
type basicEntry struct {
	started  string //no reason to convert to uint64
	duration string
	cmd      string
}

var boringCommands = strings.Join([]string{
	"cd",
	"ls",
	"top",
	"htop",
}, ",")

//location of database file
var databaseFile string

//location of history file
var historyFile string

func init() {
	host, err := os.Hostname()
	if err != nil {
		host = "UNKNOWN"
	}
	flag.StringVar(&databaseFile, "database", filepath.Join(homeDir, ".histdb/zsh-history.db"),
		"location of database file")
	flag.StringVar(&historyFile, "history", filepath.Join(homeDir, ".zsh_history"),
		"location of history file")
	flag.StringVar(&boringCommands, "ignore", boringCommands, "commands to ignore during import")
	flag.StringVar(&hostName, "host", host, "value for host column")
}

//Reads the entry, traversing multiple lines if needed
func readEntry(s *bufio.Scanner) (string, bool) {
	var ok bool
	entry := ""
	for {
		ok = s.Scan()
		if !ok {
			break
		}
		entry += s.Text()
		entryLen := len(entry)
		if entryLen == 0 {
			break
		}
		//multiline cmds end with slash
		if entry[entryLen-1] == '\\' {
			//trim the slash and restore the new line
			entry = entry[:entryLen-1] + "\n"
			continue
		}
		break
	}
	return entry, ok
}

//Parses an entry string into a basicEntry
func parseEntry(entry string) (basicEntry, error) {
	data := strings.SplitN(entry, ";", 2)
	if data == nil || len(data) != 2 {
		return basicEntry{}, errors.New("Unable to parse entry= " + entry)
	}
	info := strings.Split(data[0], ":")
	if info == nil || len(info) != 3 {
		return basicEntry{}, errors.New("Unable to parse timestamp=" + data[0])
	}
	stamp := strings.TrimSpace(info[1])
	duration := strings.TrimSpace(info[2])
	cmd := data[1]
	return basicEntry{
		started:  stamp,
		duration: duration,
		cmd:      cmd,
	}, nil
}

func insertEntry(db *sql.DB, entry basicEntry) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	st, err := tx.Prepare("INSERT INTO commands (argv) VALUES (?);")
	if err != nil {
		return err
	}
	_, err = st.Exec(entry.cmd)
	if err != nil {
		return err
	}
	st, err = tx.Prepare("INSERT INTO places (host,dir) VALUES (?, ?);")
	_, err = st.Exec(hostName, homeDir)
	if err != nil {
		return err
	}
	st, err = tx.Prepare(`INSERT INTO history
	(session, command_id, place_id, exit_status, start_time, duration)
	SELECT ?, commands.rowid, places.rowid, ?, ?, ?
	FROM commands, places
	WHERE commands.argv = ? AND places.host = ? AND places.dir = ?;`)
	_, err = st.Exec(sessionNum, retVal, entry.started, entry.duration,
		entry.cmd, hostName, homeDir)
	if err != nil {
		return err
	}
	return tx.Commit()
	/*
	   insert into commands (argv) values (${cmd});
	   insert into places   (host, dir) values (${HISTDB_HOST}, ${pwd});
	   insert into history
	     (session, command_id, place_id, exit_status, start_time, duration)
	   select
	     ${HISTDB_SESSION},
	     commands.rowid,
	     places.rowid,
	     ${retval},
	     ${started},
	     ${now} - ${started}
	   from
	     commands, places
	   where
	     commands.argv = ${cmd} and
	     places.host = ${HISTDB_HOST} and
	     places.dir = ${pwd}
	   ;
	*/
}

func main() {
	flag.Parse()

	db, err := sql.Open("sqlite3", databaseFile)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	fd, err := os.Open(historyFile)
	if err != nil {
		log.Fatal(err)
	}
	defer fd.Close()

	r := transform.NewReader(fd, unicode.UTF8.NewDecoder())
	scanner := bufio.NewScanner(r)

	bcs := strings.Split(boringCommands, ",")

outer:
	for {
		entry, ok := readEntry(scanner)
		if !ok {
			break
		}
		if entry == "" {
			continue
		}

		err = scanner.Err()
		if err != nil {
			log.Fatal(err)
		}

		parsed, err := parseEntry(entry)
		if err != nil {
			log.Fatal(err)
		}

		for _, bc := range bcs {
			if parsed.cmd == bc {
				log.Printf("Skipping %+v\n", parsed)
				continue outer
			}
		}

		log.Printf("Inserting %+v\n", parsed)
		err = insertEntry(db, parsed)
		if err != nil {
			log.Fatal(err)
		}
	}

}
