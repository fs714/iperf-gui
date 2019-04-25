package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type IperfResult struct {
	Index    int64
	Rate     float64
	Jitter   float64
	Total    int64
	Lost     int64
	LossRate float64
}

type IperfResults struct {
	IRs []IperfResult
	Mu  sync.Mutex
}

var Results IperfResults

func init() {
	var version bool
	flag.BoolVar(&version, "v", false, "Version")
	flag.Parse()

	if version {
		fmt.Println(AppVersion)
		os.Exit(0)
	}
}

func RateHandler(w http.ResponseWriter, r *http.Request) {
	var result [][]string
	for _, res := range Results.IRs {
		result = append(result, []string{strconv.FormatInt(res.Index, 10), strconv.FormatFloat(res.Rate, 'E', -1, 64)})
	}

	resJson, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resJson)
}

func JitterHandler(w http.ResponseWriter, r *http.Request) {
	var result [][]string
	for _, res := range Results.IRs {
		result = append(result, []string{strconv.FormatInt(res.Index, 10), strconv.FormatFloat(res.Jitter, 'E', -1, 64)})
	}

	resJson, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resJson)
}

func LossRateHandler(w http.ResponseWriter, r *http.Request) {
	var result [][]string
	for _, res := range Results.IRs {
		result = append(result, []string{strconv.FormatInt(res.Index, 10), strconv.FormatFloat(res.LossRate, 'E', -1, 64)})
	}

	resJson, err := json.Marshal(result)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resJson)
}

func TcpSrvResultHandler(m string) (r IperfResult, err error) {
	rs := strings.Fields(m)

	var index float64
	index, err = strconv.ParseFloat(strings.Split(rs[2], "-")[0], 64)
	if err != nil {
		fmt.Printf("Failed to parse %s to int64\n", rs[2])
		return
	}
	r.Index = int64(index)

	var rate float64
	rate, err = strconv.ParseFloat(rs[6], 64)
	if err != nil {
		fmt.Printf("Failed to parse %s to int64\n", rs[6])
		return
	}
	r.Rate = rate

	return
}

func UdpSrvResultHandler(m string) (r IperfResult, err error) {
	rs := strings.Fields(m)

	var index float64
	index, err = strconv.ParseFloat(strings.Split(rs[2], "-")[0], 64)
	if err != nil {
		fmt.Printf("Failed to parse %s to int64\n", rs[2])
		return
	}
	r.Index = int64(index)

	var rate float64
	rate, err = strconv.ParseFloat(rs[6], 64)
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

func main() {
	go func() {
		fmt.Println("Start HTTP Server http://192.168.56.101:4096/")
		http.HandleFunc("/rate", RateHandler)
		http.HandleFunc("/jitter", JitterHandler)
		http.HandleFunc("/lossrate", LossRateHandler)
		http.Handle("/", http.StripPrefix("/", http.FileServer(assetFS())))

		err := http.ListenAndServe(":4096", nil)
		if err != nil {
			fmt.Println("Failed to start http server with error: " + err.Error())
			os.Exit(0)
		}
	}()

	iperf3, err := exec.LookPath("iperf3")
	if err != nil {
		fmt.Println("The iperf3 is not installed")
		os.Exit(0)
	}

	args := "--forceflush -s -i 1 -f k"
	cmd := exec.Command(iperf3, strings.Split(args, " ")...)
	defer func() {
		_ = cmd.Process.Kill()
	}()

	stdout, _ := cmd.StdoutPipe()

	scanner := bufio.NewScanner(stdout)
	scanner.Split(bufio.ScanLines)

	err = cmd.Start()
	if err != nil {
		fmt.Println(err)
		_ = cmd.Process.Kill()
		os.Exit(0)
	}

	mode := "tcp"
	index := 0
	for scanner.Scan() {
		m := scanner.Text()
		fmt.Println(m)

		if strings.Contains(m, "Bitrate") {
			if strings.Contains(m, "Jitter") {
				mode = "udp"
			} else {
				mode = "tcp"
			}
		}

		if strings.Contains(m, " sec ") && !strings.Contains(m, "receiver") {
			var r IperfResult
			if mode == "tcp" {
				r, err = TcpSrvResultHandler(m)

			} else if mode == "udp" {
				r, err = UdpSrvResultHandler(m)
			}

			if err != nil {
				fmt.Println(err)
				_ = cmd.Process.Kill()
				os.Exit(0)
			}

			r.Index = int64(index)
			index++

			Results.Mu.Lock()
			Results.IRs = append(Results.IRs, r)
			Results.Mu.Unlock()
		}
	}

	_ = cmd.Wait()
}
