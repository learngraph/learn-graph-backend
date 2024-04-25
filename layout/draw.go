package layout

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

func drawGraph(nodes []*Node, filename string, invertColor bool) error {
	width := 800
	height := 600
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	background := color.White
	if invertColor {
		background = color.Black
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, background)
		}
	}
	nodeColor := color.Black
	if invertColor {
		nodeColor = color.White
	}
	for _, node := range nodes {
		// TODO: scale position down to fit into the image -> find min/max position values first
		img.Set(int(node.Pos.X()), int(node.Pos.Y()), nodeColor)
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	err = png.Encode(file, img)
	if err != nil {
		return err
	}
	return nil
}
