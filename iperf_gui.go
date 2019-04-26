package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
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

var version bool
var isIperf2 bool
var isUdp bool
var port int

func init() {
	flag.BoolVar(&version, "v", false, "Version")
	flag.BoolVar(&isIperf2, "2", false, "Use Iperf2")
	flag.BoolVar(&isUdp, "u", false, "Use UDP with Iperf2")
	flag.IntVar(&port, "p", 4096, "Http server listening port")
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
	reg, _ := regexp.Compile(`.*sec\s+\S+\s+KBytes\s+(\S+)\s+Kbits.*`)
	if reg.MatchString(m) {
		var rate float64
		rate, err = strconv.ParseFloat(reg.FindStringSubmatch(m)[1], 64)
		if err != nil {
			fmt.Printf("Failed to parse %s to float64\n", reg.FindStringSubmatch(m)[1])
			return
		}
		r.Rate = rate
	} else {
		err = errors.New("Failed to match: " + m)
		fmt.Println(err.Error())
	}

	return
}

func UdpSrvResultHandler(m string) (r IperfResult, err error) {
	reg, _ := regexp.Compile(`.*sec\s+\S+\s+KBytes\s+(\S+)\s+Kbits/sec\s+(\S+)\s+ms\s+(\S+)/\s*(\S+)\s+\((\S+)%\).*`)
	if reg.MatchString(m) {
		var rate float64
		rate, err = strconv.ParseFloat(reg.FindStringSubmatch(m)[1], 64)
		if err != nil {
			fmt.Printf("Failed to parse %s to float64\n", reg.FindStringSubmatch(m)[1])
			return
		}
		r.Rate = rate

		var jitter float64
		jitter, err = strconv.ParseFloat(reg.FindStringSubmatch(m)[2], 64)
		if err != nil {
			fmt.Printf("Failed to parse %s to float64\n", reg.FindStringSubmatch(m)[2])
			return
		}
		r.Jitter = jitter

		var lost int64
		lost, err = strconv.ParseInt(reg.FindStringSubmatch(m)[3], 10, 64)
		if err != nil {
			fmt.Printf("Failed to parse %s to int64\n", reg.FindStringSubmatch(m)[3])
			return
		}
		r.Lost = lost

		var total int64
		total, err = strconv.ParseInt(reg.FindStringSubmatch(m)[4], 10, 64)
		if err != nil {
			fmt.Printf("Failed to parse %s to int64\n", reg.FindStringSubmatch(m)[4])
			return
		}
		r.Total = total

		var lossRate float64
		lossRate, err = strconv.ParseFloat(reg.FindStringSubmatch(m)[5], 64)
		if err != nil {
			fmt.Printf("Failed to parse %s to float64\n", reg.FindStringSubmatch(m)[5])
			return
		}
		r.LossRate = lossRate
	} else {
		err = errors.New("Failed to match: " + m)
		fmt.Println(err.Error())
	}

	return
}

func main() {
	go func() {
		fmt.Printf("Start HTTP Server on port %d\n", port)
		http.HandleFunc("/rate", RateHandler)
		http.HandleFunc("/jitter", JitterHandler)
		http.HandleFunc("/lossrate", LossRateHandler)
		http.Handle("/", http.StripPrefix("/", http.FileServer(assetFS())))

		err := http.ListenAndServe(":"+strconv.Itoa(port), nil)
		if err != nil {
			fmt.Println("Failed to start http server with error: " + err.Error())
			os.Exit(0)
		}
	}()

	var cmd *exec.Cmd
	var err error
	if isIperf2 {
		iperf2, err := exec.LookPath("iperf")
		if err != nil {
			fmt.Println("The iperf2 is not installed")
			os.Exit(0)
		}

		var args string
		if isUdp {
			args = "-s -u -i 1 -f k"
		} else {
			args = "-s -i 1 -f k"
		}

		cmd = exec.Command(iperf2, strings.Split(args, " ")...)
	} else {
		iperf3, err := exec.LookPath("iperf3")
		if err != nil {
			fmt.Println("The iperf3 is not installed")
			os.Exit(0)
		}

		args := "--forceflush -s -i 1 -f k"
		cmd = exec.Command(iperf3, strings.Split(args, " ")...)
	}

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

		if strings.Contains(m, "Interval") {
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
				continue
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
