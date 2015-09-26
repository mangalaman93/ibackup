package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"time"
)

import influxdb "github.com/influxdb/influxdb/client"

const (
	BTFILE = ".influxdb.last"
)

func main() {
	var host, user, pass, database, dest string
	flag.StringVar(&host, "host", "localhost:8086", "<ip:port>")
	flag.StringVar(&user, "username", "root", "username")
	flag.StringVar(&pass, "password", "root", "password")
	flag.StringVar(&database, "database", "", "database to dump")
	flag.StringVar(&dest, "o", "", "destination dir")
	flag.Parse()

	url, err := url.Parse(fmt.Sprintf("http://%s", host))
	if err != nil {
		fmt.Println("Unable to parse ", host)
		os.Exit(1)
	}

	if database == "" {
		fmt.Println("database required!")
		flag.Usage()
		os.Exit(1)
	}

	if dest == "" {
		fmt.Println("destination dir required!")
		flag.Usage()
		os.Exit(1)
	}

	// create the destination dir if it doesn't already exist
	_, err = os.Stat(dest)
	if err != nil {
		err = os.Mkdir(dest, 0777)
		if err != nil {
			fmt.Println("unable to create/find destination dir, ", err)
			os.Exit(1)
		}
	}

	// check for last backup timestamp file
	var lasttime int64 = 0
	var file_created bool = false
	btfilepath := path.Join(dest, BTFILE)
	_, err = os.Stat(btfilepath)
	if err != nil {
		_, err := os.Create(btfilepath)
		if err != nil {
			fmt.Printf("unable to create file in %s, %s\n", dest, err)
			os.Exit(1)
		}
		file_created = true
	} else {
		timestr, err := ioutil.ReadFile(btfilepath)
		if err != nil {
			fmt.Printf("unable to read file in %s, %s\n", dest, err)
			os.Exit(1)
		}

		lasttime, err = strconv.ParseInt(strings.TrimRight(string(timestr), "\n"), 10, 64)
		if err != nil {
			fmt.Printf("unable to parse file %s, %s\n", btfilepath, err)
			fmt.Printf("delete %s file and try again!\n", btfilepath)
			fmt.Println("this will back up the database from the beginning!")
			os.Exit(1)
		}
	}

	// influxdb
	client, err := influxdb.NewClient(influxdb.Config{
		URL:      *url,
		Username: user,
		Password: pass,
	})
	if err != nil {
		fmt.Println("unable to create influxdb client, ", err)
		os.Exit(1)
	}

	// flag for transaction
	var commit bool = false

	// create directory for backup
	curtime := time.Now()
	dirname := path.Join(dest, fmt.Sprintf("%d_%02d_%02d_%02d_%02d_%02d",
		curtime.Year(), curtime.Month(), curtime.Day(),
		curtime.Hour(), curtime.Minute(), curtime.Second()))
	_, err = os.Stat(dirname)
	if err != nil {
		err = os.Mkdir(dirname, 0777)
		if err != nil {
			fmt.Println("unable to create/find destination dir, ", err)
			os.Exit(1)
		}
	}

	// undo if the backup operation is incomplete
	defer func() {
		if !commit {
			// delete the backup directory
			err := os.RemoveAll(dirname)
			if err != nil {
				fmt.Printf("unable to delete dir, delete %s manually!\n", dirname)
			}

			if file_created {
				// delete the backup root directory
				err := os.RemoveAll(dest)
				if err != nil {
					fmt.Printf("unable to delete dir, delete %s manually!\n", dest)
				}
			}
		}
	}()

	// list measurements
	query := influxdb.Query{
		Command:  "SHOW MEASUREMENTS",
		Database: database,
	}
	response, err := client.Query(query)
	if err != nil {
		panic(fmt.Sprintln("error querying database, ", err))
	}
	if response.Error() != nil {
		panic(fmt.Sprintln("error querying database, ", response.Error()))
	}
	measurements := response.Results[0].Series[0].Values

	// write each measurement data to file
	for _, series := range measurements {
		download_file := path.Join(dirname, fmt.Sprintf("%s.json", series[0]))
		fmt.Printf("downloading %s into %s\n", series[0], download_file)
		out, err := exec.Command("curl", "-H", fmt.Sprintf("Host:%s", host), "-G",
			fmt.Sprintf("http://%s/query", host), "-u", user+":"+pass,
			"--data-urlencode", fmt.Sprintf("db=%s", database), "--data-urlencode",
			fmt.Sprintf("q=SELECT * FROM %s where time > %d", series[0], lasttime),
			"-o", download_file).CombinedOutput()
		if err != nil {
			fmt.Println("stdout: ", string(out))
			fmt.Println("stderr: ", err)
			panic("incomplete download!")
		}
	}

	// write to the btfile finally!
	err = ioutil.WriteFile(btfilepath,
		[]byte(fmt.Sprintln(int32(time.Now().Unix()))), 0777)
	if err != nil {
		panic(fmt.Sprintf("unable to write to file in %s, %s\n", dest, err))
	}

	commit = true
}
