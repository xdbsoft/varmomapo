// Package heatmap generates heatmaps for map overlays.
package heatmap

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync"
)

// Heatmap draws a heatmap.
//
// size is the size of the image to create
// dotSize is the impact size of each point on the output
// opacity is the alpha value (0-255) of the impact of the image overlay
// scheme is the color palette to choose from the overlay
func Heatmap(size image.Rectangle, points []image.Point, limits image.Rectangle, dotSize int, opacity uint8,
	scheme []color.Color) *image.RGBA {

	dot := mkDot(float64(dotSize))

	// Draw black/alpha into the image
	bw := image.NewRGBA(size)
	placePoints(size, limits, bw, points, dot)

	rv := image.NewRGBA(size)

	// Then we transplant the pixels one at a time pulling from our color map
	warm(rv, bw, opacity, scheme)
	return rv
}

func placePoints(size image.Rectangle, limits image.Rectangle,
	bw *image.RGBA, points []image.Point, dot draw.Image) {
	for _, p := range points {
		placePoint(limits, p, bw, dot)
	}
}

func warm(out, in draw.Image, opacity uint8, colors []color.Color) {
	draw.Draw(out, out.Bounds(), image.Transparent, image.ZP, draw.Src)
	bounds := in.Bounds()
	collen := float64(len(colors))
	wg := &sync.WaitGroup{}
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		wg.Add(1)
		go func(x int) {
			defer wg.Done()
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				col := in.At(x, y)
				_, _, _, alpha := col.RGBA()
				if alpha > 0 {
					percent := float64(alpha) / float64(0xffff)
					template := colors[int((collen-1)*(1.0-percent))]
					tr, tg, tb, ta := template.RGBA()
					ta /= 256
					outalpha := uint8(float64(ta) *
						(float64(opacity) / 256.0))
					outcol := color.NRGBA{
						uint8(tr / 256),
						uint8(tg / 256),
						uint8(tb / 256),
						uint8(outalpha)}
					out.Set(x, y, outcol)
				}
			}
		}(x)
	}
	wg.Wait()
}

func mkDot(size float64) draw.Image {
	i := image.NewRGBA(image.Rect(0, 0, int(size), int(size)))

	md := 0.5 * math.Sqrt(math.Pow(float64(size)/2.0, 2)+math.Pow((float64(size)/2.0), 2))
	for x := float64(0); x < size; x++ {
		for y := float64(0); y < size; y++ {
			d := math.Sqrt(math.Pow(x-size/2.0, 2) + math.Pow(y-size/2.0, 2))
			if d < md {
				rgbVal := uint8(200.0*d/md + 50.0)
				rgba := color.NRGBA{0, 0, 0, 255 - rgbVal}
				i.Set(int(x), int(y), rgba)
			}
		}
	}

	return i
}

func translate(l image.Rectangle, p image.Point, i draw.Image, dotsize int) (rv image.Point) {
	rv.X = p.X - l.Min.X - dotsize/2
	rv.Y = i.Bounds().Max.Y - p.Y + l.Min.Y - dotsize/2
	return
}

func placePoint(l image.Rectangle, p image.Point, i, dot draw.Image) {
	pos := translate(l, p, i, dot.Bounds().Max.X)
	dotw, doth := dot.Bounds().Max.X, dot.Bounds().Max.Y
	draw.Draw(i, image.Rect(pos.X, pos.Y, pos.X+dotw, pos.Y+doth), dot,
		image.Point{X: 0, Y: 0}, draw.Over)
}
