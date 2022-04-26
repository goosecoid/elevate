package main

import (
	"os"

	"github.com/tkrajina/gpxgo/gpx"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

type DataPoint struct {
	Latitude              float64
	Longitude             float64
	Elevation             gpx.NullableFloat64
	Accumulated3dDistance float64
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

	p := plot.New()

	p.Title.Text = "Elevation Profile"
	p.X.Label.Text = "Elevation (m)"
	p.Y.Label.Text = "Distance (m)"

	var plotPoints plotter.XYs

	for _, dataPoint := range dataPoints {
		plotPoints = append(plotPoints, plotter.XY{
			X: dataPoint.Accumulated3dDistance,
			Y: dataPoint.Elevation.Value(),
		})
	}

	err = plotutil.AddLinePoints(p, "Elevation", plotPoints)

	if err != nil {
		panic(err)
	}

	if err := p.Save(16*vg.Inch, 8*vg.Inch, "points.png"); err != nil {
		panic(err)
	}
}
