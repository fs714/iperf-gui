package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type TcpSrvResult struct {
	Count int64
	Rate  int64
}

type UdpSrvResult struct {
	Count    int64
	Rate     int64
	Jitter   float64
	Total    int64
	Lost     int64
	LossRate float64
}

func init() {
	var version bool
	flag.BoolVar(&version, "v", false, "Version")
	flag.Parse()

	if version {
		fmt.Println(AppVersion)
		os.Exit(0)
	}
}

func main() {
	iperf3, err := exec.LookPath("iperf3")
	if err != nil {
		fmt.Println("The iperf3 is not installed")
		os.Exit(0)
	}

	args := "--forceflush -s -i 1 -f k"
	cmd := exec.Command(iperf3, strings.Split(args, " ")...)

	stdout, _ := cmd.StdoutPipe()

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)

	err = cmd.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}

	mode := "tcp"
	for scanner.Scan() {
		m := scanner.Text()

		if strings.Contains(m, "Jitter") {
			mode = "udp"
		}

		if strings.Contains(m, " sec ") {
			if mode == "tcp" {
				r, err := TcpSrvResultHandler(m)
				if err != nil {
					fmt.Println(err)
					os.Exit(0)
				}

				fmt.Println(r)
			} else if mode == "udp" {
				r, err := UdpSrvResultHandler(m)
				if err != nil {
					fmt.Println(err)
					os.Exit(0)
				}

				fmt.Println(r)
			}
		}
	}

	_ = cmd.Wait()
}

func TcpSrvResultHandler(m string) (r TcpSrvResult, err error) {
	rs := strings.Fields(m)

	var cntf float64
	cntf, err = strconv.ParseFloat(strings.Split(rs[2], "-")[0], 64)
	if err != nil {
		fmt.Printf("Failed to parse %s to int64\n", rs[2])
		return
	}
	r.Count = int64(cntf)

	var rate int64
	rate, err = strconv.ParseInt(rs[6], 10, 64)
	if err != nil {
		fmt.Printf("Failed to parse %s to int64\n", rs[6])
		return
	}
	r.Rate = rate

	return
}

func UdpSrvResultHandler(m string) (r UdpSrvResult, err error) {
	rs := strings.Fields(m)

	var cntf float64
	cntf, err = strconv.ParseFloat(strings.Split(rs[2], "-")[0], 64)
	if err != nil {
		fmt.Printf("Failed to parse %s to int64\n", rs[2])
		return
	}
	r.Count = int64(cntf)

	var rate int64
	rate, err = strconv.ParseInt(rs[6], 10, 64)
	if err != nil {
		fmt.Printf("Failed to parse %s to int64\n", rs[6])
		return
	}
	r.Rate = rate

	var jitter float64
	jitter, err = strconv.ParseFloat(rs[8], 64)
	if err != nil {
		fmt.Printf("Failed to parse %s to float64\n", rs[8])
		return
	}
	r.Jitter = jitter

	var total int64
	total, err = strconv.ParseInt(strings.Split(rs[10], "/")[0], 10, 64)
	if err != nil {
		fmt.Printf("Failed to parse %s to int64\n", rs[10])
		return
	}
	r.Total = total

	var lost int64
	lost, err = strconv.ParseInt(strings.Split(rs[10], "/")[1], 10, 64)
	if err != nil {
		fmt.Printf("Failed to parse %s to int64\n", rs[10])
		return
	}
	r.Lost = lost

	var lossRate float64
	lossRate, err = strconv.ParseFloat(rs[11][1:len(rs[11])-2], 64)
	if err != nil {
		fmt.Printf("Failed to parse %s to float64\n", rs[8])
		return
	}
	r.LossRate = lossRate

	return
}
