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
	"gonum.org/v1/plot/vg/draw"
)

func findNearestMultiple(input, divisor int) int {
	return (input + divisor) - (input % divisor)
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

// XYZer wraps the Len and XYZ methods.
type XYZer interface {
	Len() int
	XYZ(int) (float64, float64, float64)
}

// XYZs implements the XYZer interface using a slice.
type XYZs []struct{ X, Y, Z float64 }

// Len implements the Len method of the XYZer interface.
func (xyz XYZs) Len() int {
	return len(xyz)
}

// XYZ implements the XYZ method of the XYZer interface.
func (xyz XYZs) XYZ(i int) (float64, float64, float64) {
	return xyz[i].X, xyz[i].Y, xyz[i].Z
}

// CopyXYZs copies an XYZer, plot always requeires a copy of the data
func CopyXYZs(data XYZer) XYZs {
	cpy := make(XYZs, data.Len())
	for i := range cpy {
		cpy[i].X, cpy[i].Y, cpy[i].Z = data.XYZ(i)
	}
	return cpy
}

type Profile struct {
	XYZs
	draw.LineStyle
	StepWidth           float64
	LineWidth           vg.Length
	yellow, orange, red color.Color
	gradientIntervals   float64
}

func NewProfile(
	data XYZer, yellow, orange, red color.Color,
	gradientIntervals, stepWidth float64) *Profile {

	cpy := CopyXYZs(data)

	return &Profile{
		XYZs:              cpy,
		LineStyle:         plotter.DefaultLineStyle,
		yellow:            yellow,
		orange:            orange,
		red:               red,
		gradientIntervals: gradientIntervals,
		LineWidth:         plotter.DefaultLineStyle.Width,
		StepWidth:         stepWidth,
	}
}

type DataPoint struct {
	Latitude              float64
	Longitude             float64
	Elevation             gpx.NullableFloat64
	Accumulated3dDistance float64
	InterpolatedGradient  float64
}

type CustomTicks struct {
	Interval int
}

var (
	dataPoints     []DataPoint
	elevationSlice []float64
	plotPoints     plotter.XYs
	plotPointsz    XYZs
)

func gpx3dDistanceHelper(dps []DataPoint, i int) float64 {
	return gpx.Distance3D(dps[i].Latitude, dps[i].Longitude,
		dps[i].Elevation, dps[i+1].Latitude, dps[i+1].Longitude, dps[i+1].Elevation, true)
}

func calculateInterpolatedGradient(dps []DataPoint, i int) float64 {
	return ((dps[i+1].Elevation.Value() - dps[i].Elevation.Value()) /
		gpx3dDistanceHelper(dps, i)) * 100
}

func parseGpxToDesiredCsvDataValues(gpxFile *gpx.GPX) []DataPoint {
	var dPoints []DataPoint

	for _, track := range gpxFile.Tracks {
		for _, segment := range track.Segments {
			for _, point := range segment.Points {
				dPoints = append(dPoints, DataPoint{
					Latitude:  point.Latitude,
					Longitude: point.Longitude,
					Elevation: point.Elevation,
				})
			}
		}
	}

	for _, track := range gpxFile.Tracks {
		for _, segment := range track.Segments {
			for i := 0; i < len(segment.Points)-1; i++ {
				if i == 0 {
					dPoints[i].Accumulated3dDistance = 0
					dPoints[i].InterpolatedGradient = math.NaN()
				}
				dPoints[i+1].Accumulated3dDistance = gpx3dDistanceHelper(dPoints, i) + dPoints[i].Accumulated3dDistance
				dPoints[i+1].InterpolatedGradient = calculateInterpolatedGradient(dPoints, i)
			}
		}
	}

	return dPoints
}

func (c CustomTicks) Ticks(min, max float64) []plot.Tick {
	var tks []plot.Tick
	interval := c.Interval
	start := min

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

func (pr *Profile) Plot(c draw.Canvas, plt *plot.Plot) {
	trX, trY := plt.Transforms(&c)
	lineStyle := pr.LineStyle
	steps := findNearestMultiple(pr.XYZs.Len(), int(pr.StepWidth))

	for i := 0; i < steps; {
		d := pr.XYZs[i]
		x := trX(d.X)
		y := trY(d.Y)

		var averageGradient float64
		next := int(math.Min(float64(pr.Len())-pr.StepWidth-1, float64(i)+pr.StepWidth))
		for j := 0; j <= next-i; j++ {
			if j == (next-i) {
				averageGradient = averageGradient / float64(next-i)
				break
			}
			averageGradient = averageGradient + pr.XYZs[i+j].Z
		}

		if int(averageGradient) >= 2 && int(averageGradient) < 5 {
			lineStyle.Color = pr.yellow
		} else if int(averageGradient) >= 5 && int(averageGradient) < 10 {
			lineStyle.Color = pr.orange
		} else if int(averageGradient) >= 10 && int(averageGradient) < 15 {
			lineStyle.Color = pr.red
		} else if math.IsNaN(averageGradient) || int(averageGradient) < 2 {
			lineStyle.Color = color.Transparent
		} else {
			lineStyle.Color = color.Black
		}

		for j := 0; j <= next-i; j++ {
			if j == (next - i) {
				averageGradient = averageGradient / float64(next-i)
				break
			}
			averageGradient = averageGradient + pr.XYZs[i+j].Z
		}

		for j := 0; j <= next-i; j++ {
			dNext := pr.XYZs[i + j + 1]
			xNext := trX(dNext.X)
			yNext := trY(dNext.Y)
			poly := c.ClipPolygonY([]vg.Point{
				{X: x, Y: 0}, {X: xNext, Y: 0},
				{X: xNext, Y: yNext}, {X: x, Y: y}})
			c.FillPolygon(lineStyle.Color, poly)
			c.StrokeLines(lineStyle, poly)
		}

		i = i + int(pr.StepWidth)
		if i >= pr.XYZs.Len() {
			break
		}
	}
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

	dataPoints = parseGpxToDesiredCsvDataValues(gpxFile)

	for j := 0; j < len(dataPoints); j++ {
		elevationSlice = append(elevationSlice, dataPoints[j].Elevation.Value())
	}

	elevationSlice = sort.Float64Slice(elevationSlice)

	p := plot.New()
	p.Y.Label.Text = "Elevation (m)"
	p.Y.Min = 0
	p.Y.Max = 2000
	p.Y.Tick.Marker = CustomTicks{Interval: 100}
	p.X.Min = 0
	p.X.Max = dataPoints[len(dataPoints)-1].Accumulated3dDistance
	p.X.Label.Text = "Distance (m)"
	p.X.Tick.Marker = CustomTicks{Interval: 1000}
	p.HideX()
	p.HideY()

	for _, dataPoint := range dataPoints {
		plotPoints = append(plotPoints, plotter.XY{
			X: dataPoint.Accumulated3dDistance,
			Y: dataPoint.Elevation.Value(),
		})
	}

	for _, dataPoint := range dataPoints {
		plotPointsz = append(plotPointsz, plotter.XYZ{
			X: dataPoint.Accumulated3dDistance,
			Y: dataPoint.Elevation.Value(),
			Z: dataPoint.InterpolatedGradient,
		})
	}

	y, _ := ParseHexColor("#ffff33")
	o, _ := ParseHexColor("#ffb233")
	r, _ := ParseHexColor("#ff4f33")

	pr := NewProfile(plotPointsz, y, o, r, 100, 20)
	p.Add(pr)

	lpLine, lpPoints, err := plotter.NewLinePoints(plotPoints)
	if err != nil {
		panic(err)
	}

	lpLine.Color = color.Black
	lpLine.LineStyle.Width = plotter.DefaultLineStyle.Width * 3
	lpPoints.Color = color.Transparent
	p.Add(lpLine, lpPoints)

	if err := p.Save(16*vg.Inch, 8*vg.Inch, "points.png"); err != nil {
		panic(err)
	}
}
