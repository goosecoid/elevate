package main

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"sort"

	"github.com/tkrajina/gpxgo/gpx"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
)

type DataPoint struct {
	Latitude              float64
	Longitude             float64
	Elevation             gpx.NullableFloat64
	Accumulated3dDistance float64
}

type CustomTicks struct {
	Interval int
}

func ParseHexColor(s string) (c color.RGBA, err error) {
    c.A = 0xff
    switch len(s) {
    case 7:
        _, err = fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
    case 4:
        _, err = fmt.Sscanf(s, "#%1x%1x%1x", &c.R, &c.G, &c.B)
        // Double the hex digits:
        c.R *= 17
        c.G *= 17
        c.B *= 17
    default:
        err = fmt.Errorf("invalid length, must be 7 or 4")

    }
    return
}

func gpx3dDistanceHelper(dps []DataPoint, i int) float64 {
	return gpx.Distance3D(dps[i-1].Latitude, dps[i-1].Longitude, dps[i-1].Elevation, dps[i].Latitude, dps[i].Longitude, dps[i].Elevation, true)
}

func calculateAccumulated3dDistance(dps []DataPoint, i int) float64 {
	switch i {
	case 0:
		return float64(0)
	case 1:
		return gpx3dDistanceHelper(dps, i)
	default:
		return dps[i-1].Accumulated3dDistance + gpx3dDistanceHelper(dps, i)
	}
}

func parseGpxToCsvData(gpxFile *gpx.GPX) []DataPoint {
	var dataPoints []DataPoint

	for _, track := range gpxFile.Tracks {
		for _, segment := range track.Segments {
			for _, point := range segment.Points {
				dataPoints = append(dataPoints, DataPoint{
					Latitude:  point.Latitude,
					Longitude: point.Longitude,
					Elevation: point.Elevation,
				})
			}
		}
	}

	for _, track := range gpxFile.Tracks {
		for _, segment := range track.Segments {
			for i := 0; i < len(segment.Points); i++ {
				dataPoints[i].Accumulated3dDistance = calculateAccumulated3dDistance(dataPoints, i)
			}
		}
	}

	return dataPoints
}

func (c CustomTicks) Ticks(min, max float64) []plot.Tick {
	interval := c.Interval
	start := min
	var tks []plot.Tick

	for start < max {
		tk := plot.Tick{
			Value: start,
			Label: fmt.Sprintf("%.f", math.RoundToEven(start)),
		}
		start += float64(interval)
		tks = append(tks, tk)
	}

	return tks
}

func main() {
	dat, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	gpxFile, err := gpx.ParseBytes(dat)
	if err != nil {
		panic(err)
	}

	dataPoints := parseGpxToCsvData(gpxFile)
	var elevationSlice []float64

	for j := 0; j < len(dataPoints); j++ {
		elevationSlice = append(elevationSlice, dataPoints[j].Elevation.Value())
	}

	elevationSlice = sort.Float64Slice(elevationSlice)

	p := plot.New()

	p.Title.Text = "Elevation Profile"

	p.Y.Label.Text = "Elevation (m)"
	p.Y.Min = 0
	p.Y.Max = elevationSlice[len(elevationSlice)-1]
	p.Y.Tick.Marker = CustomTicks{Interval: 100}

	p.X.Min = 0
	p.X.Max = dataPoints[len(dataPoints)-1].Accumulated3dDistance
	p.X.Label.Text = "Distance (m)"
	p.X.Tick.Marker = CustomTicks{Interval: 1000}

	var plotPoints plotter.XYs

	for _, dataPoint := range dataPoints {
		plotPoints = append(plotPoints, plotter.XY{
			X: dataPoint.Accumulated3dDistance,
			Y: dataPoint.Elevation.Value(),
		})
	}

	lpLine, lpPoints, err := plotter.NewLinePoints(plotPoints)
	if err != nil {
		panic(err)
	}

	blue, err := ParseHexColor("#009dff")
	if err != nil {
		panic(err)
	}

	slightlyLighterBlue, err := ParseHexColor("#0da2ff")
	if err != nil {
		panic(err)
	}

	lpLine.FillColor = blue
	lpLine.Color = slightlyLighterBlue
	lpPoints.Color = color.Transparent

	p.Add(lpLine, lpPoints)

	if err := p.Save(16*vg.Inch, 8*vg.Inch, "points.png"); err != nil {
		panic(err)
	}
}
