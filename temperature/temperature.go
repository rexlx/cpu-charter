package temperature

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	url    = flag.String("url", "http://localhost:8080/temperature", "URL to send temperature to")
	server = flag.Bool("server", false, "run as server")
	port   = flag.String("port", "8080", "port to run on")
	name   = flag.String("name", "", "name of the app")
)

type Temperature struct {
	Name string    `json:"name"`
	Temp float64   `json:"temp"`
	Time time.Time `json:"time"`
}

func main() {
	flag.Parse()
	if *server {
		log.Println("Running as server", *port)
		http.HandleFunc("/temperature", RecieveTemperatureOverHTTP)
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%v", *port), nil))
	} else {
		log.Println("Running as client", *url)
		for range time.Tick(5 * time.Second) {
			SendTemperatureOverHTTP(PrepareTemperature())
		}
	}
}

func CheckTemp() []byte {
	// Run the "sensors -f" command.
	out, err := exec.Command("sensors", "-f").Output()
	if err != nil {
		log.Fatal(err)
	}

	// Print the output of the command.
	return out
}

func ParseTemperatureOutput(output []byte) []float64 {
	var out []float64
	var i float64
	// look for lines that start with "Core" followed by some integer
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "Core") {
			parts := strings.Split(line, " ")
			if _, err := fmt.Sscanf(parts[9], "+%f°F", &i); err != nil {
				log.Fatal(err)
			} else {
				out = append(out, i)
			}
		}
	}
	return out
}

func AverageTemperature(temps []float64) float64 {
	var sum float64
	for _, temp := range temps {
		sum += temp
	}
	return sum / float64(len(temps))
}

func GetHostnameOrDie(def string) string {
	if def != "" {
		return def
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}
	return hostname
}

func NewTemperature(temp float64) *Temperature {
	return &Temperature{
		Name: GetHostnameOrDie(*name),
		Temp: temp,
		Time: time.Now(),
	}
}

func PrepareTemperature() []byte {
	out := NewTemperature(AverageTemperature(ParseTemperatureOutput(CheckTemp())))
	o, e := json.Marshal(out)
	if e != nil {
		log.Fatal(e)
	}
	return o
}

func SendTemperatureOverHTTP(t []byte) {
	// create http client
	client := &http.Client{}
	req, err := http.NewRequest(http.MethodPost, *url, strings.NewReader(string(t)))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	// send request
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
}

func RecieveTemperatureOverHTTP(w http.ResponseWriter, r *http.Request) {
	var tmp Temperature
	err := json.NewDecoder(r.Body).Decode(&tmp)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(tmp)
}
