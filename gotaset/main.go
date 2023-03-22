package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/totomz/gotrader/gotrader"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sort"
	"time"
)

var (
	stdout = log.New(os.Stdout, "", log.Ltime|log.Lshortfile)
	build  string // set by make build
)

type Service struct {
	values map[string]gotrader.TimeSerie
}

type SearchRequest struct {
	Target string `json:"target"`
}

type GrafanaQuery struct {
	Range   GrafanaQueryRange    `json:"range"`
	Targets []GrafanaQueryTarget `json:"targets"`
}

type GrafanaQueryRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type GrafanaQueryTarget struct {
	// Payload map[string]interface{} `json:"payload"`  // FIXME bug in the service
	Target string `json:"target"`
}

type GrafanaTS struct {
	Target     string      `json:"target"`
	Datapoints [][]float64 `json:"datapoints"`
}

func LoadMetrics(path string) map[string]gotrader.TimeSerie {
	file, err := os.Open(path)
	if err != nil {
		stdout.Fatalf("[ERROR] reading signals file - %v", err)
	}

	contents, err := io.ReadAll(file)
	if err != nil {
		stdout.Fatalf("[ERROR] reading signals file - %v", err)
	}

	values := map[string]gotrader.TimeSerie{}

	err = json.Unmarshal(contents, &values)
	if err != nil {
		stdout.Fatalf("[ERROR] reading signals file - %v", err)
	}

	return values
}

func main() {

	stdout.Printf("Build: %s", build)

	filePath := flag.String("file", "./plotly/signals_grafana.json", "file to watch")
	flag.Parse()

	stdout.Printf(fmt.Sprintf("lalalalalalalala: %s", *filePath))

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		stdout.Fatal(err)
	}
	defer func() { _ = watcher.Close() }()

	service := Service{
		// values: AlignMetrics(LoadMetrics("gotaset/signals_grafana.json")),
		values: LoadMetrics(*filePath),
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					stdout.Println("reloading ", event.Name)
					service.values = LoadMetrics(*filePath) // no one will never know
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				stdout.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(*filePath)
	if err != nil {
		stdout.Fatal(err)
	}

	netIp := GetOutboundIP()
	netPort := 8080
	connStr := fmt.Sprintf("0.0.0.0:%v", netPort)
	stdout.Printf("listening on %s", connStr)
	stdout.Printf("your public ip is %s", netIp)

	http.HandleFunc("/search", service.Search)
	http.HandleFunc("/query", service.Query)
	http.HandleFunc("/", service.Hello)

	err = http.ListenAndServe(connStr, nil)
	if err != nil {
		stdout.Fatalf("[ERROR] can't start web server - %v", err)
	}

}

func (s *Service) Hello(w http.ResponseWriter, r *http.Request) {
	stdout.Printf("catch! %s", r.URL.Path)
	w.WriteHeader(200)
	_, _ = io.WriteString(w, "version 1")
}

func logRequest(r *http.Request) []byte {
	request, err := io.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	println(fmt.Sprintf("=== %s ===", r.URL))
	println(string(request))
	println("=== ===")

	return request
}

// Search return a JSON array with the list of available metrics
func (s *Service) Search(w http.ResponseWriter, r *http.Request) {

	body := logRequest(r)
	query := SearchRequest{}
	err := json.Unmarshal(body, &query)
	if err != nil {
		query = SearchRequest{Target: ""}
	}

	var results []string
	switch query.Target {
	case "symbols":
		results = []string{"AMD", "TSLA", "AMZN"}

	default:
		// Get all the available metrics
		for k := range s.values {
			results = append(results, k)
		}
	}

	sort.Strings(results)
	j, _ := json.Marshal(results)
	w.WriteHeader(200)
	_, _ = io.WriteString(w, string(j))
}

func (s *Service) Query(w http.ResponseWriter, r *http.Request) {

	body := logRequest(r)

	query := GrafanaQuery{}
	err := json.Unmarshal(body, &query)
	if err != nil {
		stdout.Printf("[ERROR] invalid /query - %v", err)
		http.Error(w, "invalid query", http.StatusInternalServerError)
		return
	}

	var metrics []GrafanaTS

	for _, target := range query.Targets {
		stdout.Printf("loading %s", target.Target)
		ts, exists := s.values[target.Target]
		if !exists {
			stdout.Printf("[ERROR] serie %s not found", target.Target)
		}

		var datapoints [][]float64
		for i, inst := range ts.X {

			if query.Range.From.After(ts.X[len(ts.X)-1]) ||
				query.Range.To.Before(ts.X[0]) {
				break
			}

			if inst.Before(query.Range.From) {
				continue
			}

			if inst.After(query.Range.To) {
				continue
			}

			datapoints = append(datapoints, []float64{ts.Y[i], float64(ts.X[i].UnixMilli())})
		}

		if len(datapoints) > 0 {
			metrics = append(metrics, GrafanaTS{
				Target:     target.Target,
				Datapoints: datapoints,
			})
		}

	}

	res, err := json.Marshal(metrics)
	if err != nil {
		stdout.Printf("[ERROR] can't unmarshall response - %v", err)
		http.Error(w, "invalid query", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(200)
	_, _ = io.WriteString(w, string(res))
}

// GetOutboundIP Get preferred outbound ip of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		stdout.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
