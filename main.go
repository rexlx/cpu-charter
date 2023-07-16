package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/go-echarts/go-echarts/v2/types"

	"github.com/rexlx/performance"
)

// determines if the app runs as server or client
var (
	server = flag.Bool("server", false, "run as server")
)

// Application struct holds all the data and methods for the app
type Application struct {
	Name      string
	Url       string
	Chart     *charts.Line
	Config    *ChartConfig
	ChartData []opts.LineData
	Interface *http.Client
	Aggs      []Aggregation
	Data      []performance.CpuUsage
	Mu        sync.Mutex
}

// ChartConfig holds the configuration for the chart
type ChartConfig struct {
	Start time.Time
	Times []time.Time
	Step  time.Duration
	Count int
}

type Aggregation struct {
	Value float64
	Time  time.Time
	Name  string
}

func main() {
	flag.Parse()

	// for the API
	client := &http.Client{}
	// for the chart
	items := make([]opts.LineData, 0)

	// init the app
	app := Application{
		Name:      "FoxyBoxy",
		Url:       "http://drfright:8080/",
		Interface: client,
		Aggs:      []Aggregation{},
		Data:      []performance.CpuUsage{},
		Mu:        sync.Mutex{},
		ChartData: items,
		Chart:     charts.NewLine(),
		Config: &ChartConfig{
			Start: time.Now(),
			Times: []time.Time{},
		},
	}

	// if running in client mode
	if !*server {

		for range time.Tick(6 * time.Second) {
			// create the channel for the cpu values
			stream := make(chan []*performance.CpuUsage)
			// get the cpu values
			go performance.GetCpuValues(stream, 3)
			msg := <-stream

			out, err := json.Marshal(msg)
			if err != nil {
				panic(err)
			}
			// send the good news
			app.SendCpuValuesOverHTTP(out)

		}

	} else {
		// we are the server
		fmt.Println("receive cpu values over http")
		// setup our handlers
		http.HandleFunc("/", app.ReceiveCpuValuesOverHTTP)
		http.HandleFunc("/chart", app.ShowLineChart)

		// set up our tasks in the background
		go func() {
			for range time.Tick(30 * time.Second) {
				app.AppendLineChart()
				app.SetLineChart()
			}
		}()
		http.ListenAndServe(":8080", nil)
	}

}

// SendCpuValuesOverHTTP sends the cpu values over http
func (app *Application) SendCpuValuesOverHTTP(vals []byte) {
	req, err := http.NewRequest(http.MethodPost, app.Url, strings.NewReader(string(vals)))
	if err != nil {
		panic(err)
	}

	req.Header.Add("Content-Type", "application/json")

	resp, err := app.Interface.Do(req)
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.Status)
	defer resp.Body.Close()

}

// ReceiveCpuValuesOverHTTP receives the cpu values over http and adds them to the app data / creates an aggregation
func (app *Application) ReceiveCpuValuesOverHTTP(w http.ResponseWriter, r *http.Request) {
	app.Mu.Lock()
	defer app.Mu.Unlock()

	var tmp float64
	var agg Aggregation

	rightNow := time.Now()

	var msg []performance.CpuUsage

	err := json.NewDecoder(r.Body).Decode(&msg)
	if err != nil {
		fmt.Println(err, "continuing")
		return
	}

	app.Data = append(app.Data, msg...)

	for _, v := range msg {
		tmp += v.Usage
	}

	agg.Value = tmp / float64(len(msg))
	agg.Time = rightNow
	app.Aggs = append(app.Aggs, agg)
}

// ShowLineChart renders the line chart
func (app *Application) ShowLineChart(w http.ResponseWriter, r *http.Request) {
	app.Chart.Render(w)
}

// AppendLineChart appends the line chart
func (app *Application) AppendLineChart() {
	app.Mu.Lock()
	defer app.Mu.Unlock()

	var tmp float64

	for _, v := range app.Aggs {
		tmp += v.Value
	}
	app.ChartData = append(app.ChartData, opts.LineData{Value: tmp / float64(len(app.Aggs))})
	app.Config.Times = append(app.Config.Times, time.Now())
	fmt.Println("appending", time.Now())
	app.Aggs = nil
}

// SetLineChart sets the line chart
func (app *Application) SetLineChart() {
	app.Mu.Lock()
	defer app.Mu.Unlock()
	app.Chart.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: types.ThemeShine}),
		charts.WithTitleOpts(opts.Title{
			Title:    app.Name,
			Subtitle: "CPU Usage",
		}))
	app.Chart.SetXAxis(app.Config.Times).
		AddSeries("CPU Usage", app.ChartData).
		SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
}
