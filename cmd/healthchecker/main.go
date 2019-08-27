package main

import (
	"os"

	"github.com/prometheus/client_golang/prometheus"
	kingpin "gopkg.in/alecthomas/kingpin.v2"

	"github.com/mintel/elasticsearch-asg/internal/app/healthchecker"
)

func main() {
	app, err := healthchecker.NewApp(prometheus.DefaultRegisterer)
	if err != nil {
		panic(err)
	}
	kingpin.MustParse(app.Parse(os.Args[1:]))
	app.Main(prometheus.DefaultGatherer)
}
