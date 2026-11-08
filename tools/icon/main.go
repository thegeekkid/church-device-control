// Generate the original TV icon using only Go's standard library.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func round(x, y, left, top, right, bottom, r float64) bool {
	if x < left || x > right || y < top || y > bottom {
		return false
	}
	cx := math.Max(left+r, math.Min(x, right-r))
	cy := math.Max(top+r, math.Min(y, bottom-r))
	return (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r
}
func sample(x, y float64) color.RGBA {
	if !round(x, y, 4, 4, 124, 124, 26) {
		return color.RGBA{}
	}
	c := color.RGBA{17, 31, 48, 255}
	if round(x, y, 17, 26, 111, 91, 10) {
		c = color.RGBA{224, 239, 246, 255}
	}
	if round(x, y, 23, 32, 105, 85, 5) {
		c = color.RGBA{34, 159, 154, 255}
	}
	if round(x, y, 59, 89, 69, 103, 2) || round(x, y, 42, 101, 86, 108, 3) {
		c = color.RGBA{224, 239, 246, 255}
	}
	dx, dy := x-64, y-58
	d := math.Hypot(dx, dy)
	if d >= 14 && d <= 19 && !(y < 51 && x > 56 && x < 72) {
		c = color.RGBA{238, 255, 250, 255}
	}
	if round(x, y, 61, 36, 67, 58, 2) {
		c = color.RGBA{238, 255, 250, 255}
	}
	return c
}
func main() {
	for _, size := range []int{16, 32, 48, 64, 128, 256} {
		im := image.NewNRGBA(image.Rect(0, 0, size, size))
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				var sums [4]uint32
				for sy := 0; sy < 4; sy++ {
					for sx := 0; sx < 4; sx++ {
						c := sample((float64(x)+(float64(sx)+.5)/4)*128/float64(size), (float64(y)+(float64(sy)+.5)/4)*128/float64(size))
						sums[0] += uint32(c.R) * uint32(c.A)
						sums[1] += uint32(c.G) * uint32(c.A)
						sums[2] += uint32(c.B) * uint32(c.A)
						sums[3] += uint32(c.A)
					}
				}
				var c color.NRGBA
				if sums[3] > 0 {
					c = color.NRGBA{uint8(sums[0] / sums[3]), uint8(sums[1] / sums[3]), uint8(sums[2] / sums[3]), uint8(sums[3] / 16)}
				}
				im.SetNRGBA(x, y, c)
			}
		}
		f, e := os.Create(fmt.Sprintf("winres/icon%d.png", size))
		if e != nil {
			panic(e)
		}
		if e = png.Encode(f, im); e != nil {
			panic(e)
		}
		f.Close()
	}
}
