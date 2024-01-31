/*
    TileEx : A Tiling Pattern Extractor written in Go
    Copyright (C) 2023, Sarthak Shah (shahsarthakw@gmail.com)

    This program is free software: you can redistribute it and/or modify
    it under the terms of the GNU General Public License as published by
    the Free Software Foundation, either version 3 of the License, or
    (at your option) any later version.

    This program is distributed in the hope that it will be useful,
    but WITHOUT ANY WARRANTY; without even the implied warranty of
    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
    GNU General Public License for more details.

    You should have received a copy of the GNU General Public License
    along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/
package main

import (
  "os"
	"fmt"
	"flag"
  "sort"
	"image"
	"image/png"
	"image/draw"
	"log"
	"runtime"
	"sync"
)

type Color struct {
	R, G, B uint32
}

func frequencyPairs(arr chan int, preferFrequency bool) ([][]int, int) {
	frequencyMap := make(map[int]int)
	for num := range arr {
		frequencyMap[num]++
	}
	var pairs [][]int
  var totalFrequency int
	for num, freq := range frequencyMap {
		pairs = append(pairs, []int{num, freq})
    totalFrequency += freq
	}
  pairChoice := 0
  if preferFrequency {
    pairChoice = 1
  }
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i][pairChoice] > pairs[j][pairChoice]
	})
	return pairs, totalFrequency
}

func ArrayPeriodicity(colors []Color) int {
  n := len(colors)
  var prefixArray = make([]int, n)
  var j = 0
  for i := 1; i < n; i++ {
    for j > 0 && colors[i] != colors[j] {
      j = prefixArray[j - 1]
    }
    if colors[i] == colors[j] {
      j += 1
    }
    prefixArray[i] = j
  }
  return n - prefixArray[n - 1]
}

func processRow(img image.Image, rowIdx int, wg *sync.WaitGroup, resultRow chan <- int) {
	defer wg.Done()

	bounds := img.Bounds()
	rowColors := make([]Color, bounds.Max.X)

	for x := 0; x < bounds.Max.X; x++ {
		r, g, b, _ := img.At(x, rowIdx).RGBA()
		rowColors[x] = Color{R: r, G: g, B: b}
	}

	resultRow <- ArrayPeriodicity(rowColors)
}

func processCol(img image.Image, colIdx int, wg *sync.WaitGroup, resultCol chan <- int) {
	defer wg.Done()

	bounds := img.Bounds()
	colColors := make([]Color, bounds.Max.Y)

	for y := 0; y < bounds.Max.Y; y++ {
		r, g, b, _ := img.At(colIdx, y).RGBA()
		colColors[y] = Color{R: r, G: g, B: b}
	}

	resultCol <- ArrayPeriodicity(colColors)
}

func main() {
  var input, output string
  var rowTolerance, colTolerance float64
  var offsetX, offsetY, numProc int
  var rowPreferFrequency, colPreferFrequency bool
  flag.StringVar(&input, "input", "input.png", "The input file")
  flag.StringVar(&output, "output", "output.png", "The output file")
  flag.Float64Var(&rowTolerance, "row-tolerance", 0.1, "The minimum frequency of the row periodicity value (percent)")
  flag.Float64Var(&colTolerance, "col-tolerance", 0.1, "The minimum frequency of the col periodicity value (percent)")
  flag.IntVar(&offsetX, "x-offset", 0, "The number of pixels the width of the crop is offset by")
  flag.IntVar(&offsetY, "y-offset", 0, "The number of pixels the height of the crop is offset by")
  flag.IntVar(&numProc, "number-of-processes", runtime.NumCPU(), "The maximum number of process to be used")
  flag.BoolVar(&rowPreferFrequency, "row-prefer-frequency", false, "Give preference to the highest frequency match for rows")
  flag.BoolVar(&colPreferFrequency, "col-prefer-frequency", false, "Give preference to the highest frequency match for cols")

  flag.Parse()

  if rowPreferFrequency {
    rowTolerance = 0.0
  } else {
    rowTolerance = rowTolerance / 100.0
  }

  if colPreferFrequency {
    colTolerance = 0.0
  } else {
    colTolerance = colTolerance / 100.0
  }

	runtime.GOMAXPROCS(numProc)

	file, err := os.Open(input)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		log.Fatal(err)
	}
  
	numRows := img.Bounds().Max.Y

	var wg sync.WaitGroup
	resultRow := make(chan int, numRows)

	for y := 0; y < numRows; y++ {
		wg.Add(1)
		go processRow(img, y, &wg, resultRow)
	}

	go func() {
		wg.Wait()
		close(resultRow)
	}()

  rowPairs, rowTotalFrequency := frequencyPairs(resultRow, rowPreferFrequency)
  rowPeriodicityIdx := 0
  for rowPeriodicityIdx < len(rowPairs) && 
  rowPairs[rowPeriodicityIdx][1] < int(float64(rowTotalFrequency) * float64(rowTolerance)) {
    rowPeriodicityIdx += 1
  }
  fmt.Printf("Row periodicity is %f percent of total frequency.\n", (float64(rowPairs[rowPeriodicityIdx % len(rowPairs)][1])/float64(rowTotalFrequency))*100.0)
  rowPeriodicity := rowPairs[rowPeriodicityIdx % len(rowPairs)][0]
  fmt.Printf("Row Periodicity: %d\n", rowPeriodicity)

	numCols := img.Bounds().Max.X

	resultCol := make(chan int, numCols)

	for x := 0; x < numCols; x++ {
		wg.Add(1)
		go processCol(img, x, &wg, resultCol)
	}

	go func() {
		wg.Wait()
		close(resultCol)
	}()

  colPairs, colTotalFrequency := frequencyPairs(resultCol, colPreferFrequency)
  colPeriodicityIdx := 0
  for colPeriodicityIdx < len(colPairs) && 
  colPairs[colPeriodicityIdx][1] < int(float64(colTotalFrequency) * float64(colTolerance)) {
    colPeriodicityIdx += 1
  }
  fmt.Printf("Col periodicity is %f percent of total frequency.\n", (float64(colPairs[colPeriodicityIdx % len(colPairs)][1])/float64(colTotalFrequency))*100.0)
  colPeriodicity := colPairs[colPeriodicityIdx % len(colPairs)][0]
  fmt.Printf("Col Periodicity: %d\n", colPeriodicity)

  tileWidth := rowPeriodicity
  tileHeight := colPeriodicity
	targetImage := image.NewRGBA(image.Rect(0, 0, tileWidth, tileHeight))

  srcRect := image.Rect(offsetX, offsetY, tileWidth, tileHeight)
	dstRect := targetImage.Bounds()

	draw.Draw(targetImage, dstRect, img, srcRect.Min, draw.Src)

	outputImg, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}
	defer outputImg.Close()

	if err := png.Encode(outputImg, targetImage); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Image cropped and saved successfully.")
}
