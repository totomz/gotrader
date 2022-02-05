package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
)

var (
	stdout = log.New(os.Stdout, "", log.Ltime|log.Lshortfile)
	stderr = log.New(os.Stdout, "[ERROR]", log.Ltime|log.Lshortfile|log.Lmsgprefix)
)

func main() {

	netIp := GetOutboundIP()
	netPort := 8080
	connStr := fmt.Sprintf("%s:%v", netIp.String(), netPort)
	stdout.Printf("listening on %s", connStr)

	http.HandleFunc("/search", Search)
	http.HandleFunc("/query", Query)
	http.HandleFunc("/", Hello)

	err := http.ListenAndServe(connStr, nil)
	if err != nil {
		log.Fatal(err)
	}

}

func Hello(w http.ResponseWriter, r *http.Request) {
	stdout.Printf("catch! %s", r.URL.Path)
	w.WriteHeader(200)
	io.WriteString(w, "version 1")
}

// Search return a JSON array with the list of available metrics
func Search(w http.ResponseWriter, r *http.Request) {

	request, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	println(string(request))

	metrics := []string{"pippo", "pluto"}
	j, _ := json.Marshal(metrics)
	w.WriteHeader(200)
	io.WriteString(w, string(j))
}

func Query(w http.ResponseWriter, r *http.Request) {

	stdout.Println("/query")
	request, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	println(string(request))

	w.WriteHeader(200)
	io.WriteString(w, "version 1")
}

// GetOutboundIP Get preferred outbound ip of this machine
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
