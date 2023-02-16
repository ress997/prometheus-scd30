package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/pvainio/scd30"
	"periph.io/x/conn/v3/i2c/i2creg"
	"periph.io/x/host/v3"
)

var (
	i2c      string
	interval int
	port     string

	temperatureGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "temperature",
			Help: "Temperature measured (°C)",
		},
	)
	humidityGauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "humidity",
			Help: "Relative humidity measured (%)",
		},
	)
	co2Gauge = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "co2",
			Help: "CO₂ measured (ppm)",
		},
	)
)

func init() {
	flag.StringVar(&i2c, "i2c", "", "I²C bus to use")
	flag.IntVar(&interval, "interval", 5, "The time in seconds between CO₂ readings")
	flag.StringVar(&port, "port", ":8000", "Server Port")
	flag.Parse()
}

func main() {
	// Setup the SCD30 Sensor
	if _, err := host.Init(); err != nil {
		log.Fatal(err)
	}
	bus, err := i2creg.Open(i2c)
	if err != nil {
		log.Fatal(err)
	}
	defer bus.Close()
	sensor, err := scd30.Open(bus)
	if err != nil {
		log.Fatal(err)
	}
	sensor.StartMeasurements(uint16(interval))

	// Read the SCD30 Sensor
	go func() {
		for {
			time.Sleep(time.Duration(interval) * time.Second)

			hasMeasurement, err := sensor.HasMeasurement()
			if err != nil {
				log.Fatalf("error %v", err)
			}
			if hasMeasurement {
				m, err := sensor.GetMeasurement()
				if err != nil {
					log.Fatalf("error %v", err)
				}

				// Temp
				temperatureGauge.Set(float64(m.Temperature))

				// Hum
				humidityGauge.Set(float64(m.Humidity))

				// CO₂
				co2Gauge.Set(float64(m.CO2))

				// Console Log
				log.Printf("Temp: %.4g°C, Hum: %.3g%%, CO₂: %.4g ppm", m.Temperature, m.Humidity, m.CO2)
			} else {
				log.Print("Failed to get a measurement...")
			}
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/metrics", http.StatusFound)
	})
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Prometheus Exporter starting on port %s\n", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Println("ListenAndServe returns an  error", err)
		if err != http.ErrServerClosed {
			log.Fatalln("HTTPServer closed with error:", err)
		}
	}
}
