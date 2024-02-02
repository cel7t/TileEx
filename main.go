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
  "path"
  "fmt"
  "flag"
  "sort"
  "image"
  "image/png"
  _ "image/jpeg"
  "image/draw"
  "log"
  "math"
  "runtime"
  "sync"
)

type Color struct {
  R, G, B uint32
}

const (
  LOSSLESS = 0
  LOSSY = 1
)

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

func Gray(color Color) float64 {
  r := float64(color.R) 
  g := float64(color.G)
  b := float64(color.B) 
  return 0.299 * r +  0.587 * g + 0.114 * b
}

func ColorDiff(x, y Color) int {
  var R int = int(x.R - y.R)
  var G int = int(x.G - y.G)
  var B int = int(x.B - y.B)
  return (R*R + G*G + B*B)
}

func ArrayPeriodicityJPGPlus(colors []Color) int {
  n := len(colors)
  var minsum int
  minidx := 1
  for k := 1; k < n; k++ {
    sum := 0
    for idx, color := range colors {
      sum += ColorDiff(colors[(idx + k) % n], color)
    }
    if k == 1 {
      minsum = sum
    } else {
      if sum < minsum {
        minsum = sum
        minidx = k
      }
    }
  }
  return minidx
}

func ArrayPeriodicityJPG(colors []Color) int {
  n := len(colors)
  grayscale := make([]float64, n)
  for idx, color := range colors {
    grayscale[idx] = Gray(color)
  }
  var minsum float64
  minidx := 1
  for k := 1; k < n; k++ {
    sum := 0.0
    for idx, gray := range grayscale {
      sum += math.Abs(grayscale[(idx + k) % n] - gray)
    }
    if k == 1 {
      minsum = sum
    } else {
      if sum < minsum {
        minsum = sum
        minidx = k
      }
    }
  }
  return minidx
}

func ArrayPeriodicityPNG(colors []Color) int {
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

func processRow(img image.Image, imageFormat int, rowIdx int, wg *sync.WaitGroup, resultRow chan <- int) {
  defer wg.Done()

  bounds := img.Bounds()
  rowColors := make([]Color, bounds.Max.X)

  for x := 0; x < bounds.Max.X; x++ {
    r, g, b, _ := img.At(x, rowIdx).RGBA()
    rowColors[x] = Color{R: r, G: g, B: b}
  }

  if imageFormat == LOSSY {
    resultRow <- ArrayPeriodicityJPGPlus(rowColors)
  } else {
    resultRow <- ArrayPeriodicityPNG(rowColors)
  }
}

func processCol(img image.Image, imageFormat int, colIdx int, wg *sync.WaitGroup, resultCol chan <- int) {
  defer wg.Done()

  bounds := img.Bounds()
  colColors := make([]Color, bounds.Max.Y)

  for y := 0; y < bounds.Max.Y; y++ {
    r, g, b, _ := img.At(colIdx, y).RGBA()
    colColors[y] = Color{R: r, G: g, B: b}
  }

  if imageFormat == LOSSY {
    resultCol <- ArrayPeriodicityJPGPlus(colColors)
  } else {
    resultCol <- ArrayPeriodicityPNG(colColors)
  }
}

func main() {
  var input, output string
  var rowTolerance, colTolerance float64
  var offsetX, offsetY, numProc int
  var rowPreferFrequency, colPreferFrequency, setLossy, setLossless bool
  flag.StringVar(&input, "input", "input.png", "The input file")
  flag.StringVar(&output, "output", "output.png", "The output file")
  flag.Float64Var(&rowTolerance, "row-tolerance", 0.1, "The minimum frequency of the row periodicity value (percent)")
  flag.Float64Var(&colTolerance, "col-tolerance", 0.1, "The minimum frequency of the col periodicity value (percent)")
  flag.IntVar(&offsetX, "x-offset", 0, "The number of pixels the width of the crop is offset by")
  flag.IntVar(&offsetY, "y-offset", 0, "The number of pixels the height of the crop is offset by")
  flag.IntVar(&numProc, "number-of-processes", runtime.NumCPU(), "The maximum number of process to be used")
  flag.BoolVar(&rowPreferFrequency, "row-prefer-frequency", false, "Give preference to the highest frequency match for rows")
  flag.BoolVar(&colPreferFrequency, "col-prefer-frequency", false, "Give preference to the highest frequency match for cols")
  flag.BoolVar(&setLossy, "set-lossy", false, "Set the file type as lossy")
  flag.BoolVar(&setLossless, "set-lossless", false, "Set the file type as lossless")

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

  imageFormat := LOSSY
  if setLossy || setLossless {
    if setLossy && setLossless {
      fmt.Println("Error: Please select only one of -set-lossy or -set-lossless")
      return
    }
    if setLossless {
      imageFormat = LOSSLESS
      fmt.Println("File type: LOSSLESS")
    } else {
      fmt.Println("File type: LOSSY")
    }
  } else {
    if path.Ext(input) == ".png" {
      imageFormat = LOSSLESS
      fmt.Println("File type: LOSSLESS")
    } else {
      fmt.Println("File type: LOSSY")
    }
  }

  numRows := img.Bounds().Max.Y

  var wg sync.WaitGroup
  resultRow := make(chan int, numRows)

  for y := 0; y < numRows; y++ {
    wg.Add(1)
    go processRow(img, imageFormat, y, &wg, resultRow)
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
    go processCol(img, imageFormat, x, &wg, resultCol)
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

  srcRect := image.Rect(offsetX, offsetY, offsetX+tileWidth, offsetY+tileHeight)
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
