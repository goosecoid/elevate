package main

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"sort"

	"github.com/tkrajina/gpxgo/gpx"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/font"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/text"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
)

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
	Style                             text.Style
	StepWidth                         int
	LineWidth                         vg.Length
	white, yellow, orange, red, black color.Color
}

func NewProfile(
	data XYZer, style text.Style,
	white, yellow, orange, red, black color.Color,
	stepWidth int) *Profile {

	cpy := CopyXYZs(data)

	return &Profile{
		XYZs:      cpy,
		LineStyle: plotter.DefaultLineStyle,
		Style:     style,
		white:     white,
		yellow:    yellow,
		orange:    orange,
		red:       red,
		black:     black,
		LineWidth: plotter.DefaultLineStyle.Width,
		StepWidth: stepWidth,
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
	return gpx.Distance3D(dps[i-1].Latitude, dps[i-1].Longitude,
		dps[i-1].Elevation, dps[i].Latitude, dps[i].Longitude, dps[i].Elevation, true)
}

func calculateInterpolatedGradient(dps []DataPoint, i int) float64 {
	fmt.Print("Data: ", dps[i].Elevation.Value(), dps[i-1].Elevation.Value(), gpx3dDistanceHelper(dps, i), (dps[i].Elevation.Value() - dps[i-1].Elevation.Value()), "\n")
	return ((dps[i].Elevation.Value() - dps[i-1].Elevation.Value()) /
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
			for i := 1; i < len(segment.Points); i++ {
				if i == 1 {
					dPoints[0].Accumulated3dDistance = 0
					dPoints[0].InterpolatedGradient = math.NaN()
					dPoints[i].Accumulated3dDistance = gpx3dDistanceHelper(dPoints, i)
					dPoints[i].InterpolatedGradient = calculateInterpolatedGradient(dPoints, i)
				} else {
					dPoints[i].Accumulated3dDistance = gpx3dDistanceHelper(dPoints, i) + dPoints[i-1].Accumulated3dDistance
					dPoints[i].InterpolatedGradient = calculateInterpolatedGradient(dPoints, i)
				}
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

func (pr *Profile) drawLegend(c draw.Canvas, plt *plot.Plot) {
	var pointsToConnect []vg.Point
	trX, trY := plt.Transforms(&c)
	lineStyle := pr.LineStyle

	// Draw legend polys to show gradients
	wlegePoly := c.ClipPolygonX([]vg.Point{
		{X: trX(200), Y: trY(1100)}, {X: trX(200), Y: trY(1200)},
		{X: trX(800), Y: trY(1200)}, {X: trX(800), Y: trY(1100)}})
	c.FillPolygon(pr.white, wlegePoly)
	pointsToConnect = append(pointsToConnect, wlegePoly...)
	pointsToConnect = append(pointsToConnect, vg.Point{X: trX(200), Y: trY(1100)})
	c.StrokeLines(lineStyle, pointsToConnect)
	ylegePoly := c.ClipPolygonY([]vg.Point{
		{X: trX(200), Y: trY(1200)}, {X: trX(200), Y: trY(1300)},
		{X: trX(800), Y: trY(1300)}, {X: trX(800), Y: trY(1200)}})
	c.FillPolygon(pr.yellow, ylegePoly)
	c.StrokeLines(lineStyle, ylegePoly)
	olegePoly := c.ClipPolygonY([]vg.Point{
		{X: trX(200), Y: trY(1300)}, {X: trX(200), Y: trY(1400)},
		{X: trX(800), Y: trY(1400)}, {X: trX(800), Y: trY(1300)}})
	c.FillPolygon(pr.orange, olegePoly)
	c.StrokeLines(lineStyle, olegePoly)
	rlegePoly := c.ClipPolygonY([]vg.Point{
		{X: trX(200), Y: trY(1400)}, {X: trX(200), Y: trY(1500)},
		{X: trX(800), Y: trY(1500)}, {X: trX(800), Y: trY(1400)}})
	c.FillPolygon(pr.red, rlegePoly)
	c.StrokeLines(lineStyle, rlegePoly)
	blegePoly := c.ClipPolygonY([]vg.Point{
		{X: trX(200), Y: trY(1500)}, {X: trX(200), Y: trY(1600)},
		{X: trX(800), Y: trY(1600)}, {X: trX(800), Y: trY(1500)}})
	c.FillPolygon(pr.black, blegePoly)
	c.StrokeLines(lineStyle, blegePoly)

	c.FillText(pr.Style, vg.Point{X: trX(1500), Y: trY(1175)}, "0 - 2%")
	c.FillText(pr.Style, vg.Point{X: trX(1500), Y: trY(1275)}, "2 - 5%")
	c.FillText(pr.Style, vg.Point{X: trX(1500), Y: trY(1375)}, "5 - 10%")
	c.FillText(pr.Style, vg.Point{X: trX(1500), Y: trY(1475)}, "10 - 15%")
	c.FillText(pr.Style, vg.Point{X: trX(1500), Y: trY(1575)}, "> 15%")
}

func (pr *Profile) CalculateGradientColor(gradient float64) color.Color {
	var color color.Color

	if int(gradient) >= 2 && int(gradient) < 5 {
		color = pr.yellow
	} else if int(gradient) >= 5 && int(gradient) < 10 {
		color = pr.orange
	} else if int(gradient) >= 10 && int(gradient) < 15 {
		color = pr.red
	} else if math.IsNaN(gradient) || int(gradient) < 2 {
		color = pr.white
	} else {
		color = pr.black
	}

	return color
}

func (pr *Profile) Plot(c draw.Canvas, plt *plot.Plot) {
	gradientColors := make(map[int]draw.LineStyle)
	trX, trY := plt.Transforms(&c)
	lineStyle := pr.LineStyle
	steps := int(math.Floor(float64(pr.Len() / pr.StepWidth)))

	// Add legend
	pr.drawLegend(c, plt)

	// calculate average gradients and attribute color for polys
	for i := 1; i < pr.Len(); i++ {
		var gradientAvg float64
		gradientAvg = gradientAvg + pr.XYZs[i].Z

		if i == pr.Len() - 1 {
			gradientAvg = gradientAvg / float64(pr.Len()-(steps*pr.StepWidth))
			cl := pr.CalculateGradientColor(float64(gradientAvg))
			lineStyle.Color = cl
			for j := (steps * pr.StepWidth) + 1; j < pr.Len(); j++ {
				gradientColors[j] = lineStyle
			}
		}

		if i%int(pr.StepWidth) == 0 {
			gradientAvg = gradientAvg / float64(pr.StepWidth)
			cl := pr.CalculateGradientColor(float64(gradientAvg))
			lineStyle.Color = cl
			for j := i - int(pr.StepWidth) + 1; j <= i; j++ {
				gradientColors[j] = lineStyle
			}
		}
	}

	// draw polys
	for z := 0; z < pr.Len() - 1; z++ {
		x := trX(pr.XYZs[z].X)
		y := trY(pr.XYZs[z].Y)
		xNext := trX(pr.XYZs[z+1].X)
		yNext := trY(pr.XYZs[z+1].Y)

		poly := c.ClipPolygonXY([]vg.Point{
			{X: x, Y: 0},
			{X: xNext, Y: 0},
			{X: xNext, Y: yNext},
			{X: x, Y: y},
		})
		if z == 0 {
			c.FillPolygon(gradientColors[1].Color, poly)
			c.StrokeLines(gradientColors[1], poly)
		} else {
			c.FillPolygon(gradientColors[z].Color, poly)
			c.StrokeLines(gradientColors[z], poly)
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

	tr42 := font.Font{Variant: "Serif", Size: 26}

	p := plot.New()
	p.Title.Text = "Mont Ventoux climb from Bédoin"
	p.Title.TextStyle.YAlign = -2.5
	p.Title.TextStyle.Font = tr42
	p.Y.Min = 0
	p.Y.Max = 2000
	p.Y.Tick.Marker = CustomTicks{Interval: 100}
	p.X.Min = 0
	p.X.Max = dataPoints[len(dataPoints)-1].Accumulated3dDistance
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

	w, _ := ParseHexColor("#ffea84")
	y, _ := ParseHexColor("#ffd384")
	o, _ := ParseHexColor("#ffb684")
	r, _ := ParseHexColor("#ff9f84")
	b, _ := ParseHexColor("#ff8484")

	defaultFont := plot.DefaultFont
	hdlr := plot.DefaultTextHandler

	style := text.Style{
		Color:   color.Black,
		Font:    font.From(defaultFont, 12),
		XAlign:  draw.XCenter,
		YAlign:  draw.YTop,
		Handler: hdlr,
	}

	pr := NewProfile(plotPointsz, style, w, y, o, r, b, len(dataPoints) / 100 * 5)
	p.Add(pr)

	lpLine, lpPoints, err := plotter.NewLinePoints(plotPoints)
	if err != nil {
		panic(err)
	}

	lpLine.Color = color.Black
	lpLine.LineStyle.Width = plotter.DefaultLineStyle.Width
	lpPoints.Color = color.Transparent
	p.Add(lpLine, lpPoints)

	if err := p.Save(16*vg.Inch, 8*vg.Inch, "points.png"); err != nil {
		panic(err)
	}
}
