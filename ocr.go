package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"

	vision "cloud.google.com/go/vision/apiv1"
	"golang.org/x/net/context"
)

// detectText gets text from the Vision API for an image at the given file path.
func detectTextAndSaveToFile(outputFile string, imageUrl string) error {
	ctx := context.Background()

	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		return err
	}

	resp, err := http.Get(imageUrl)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return err
	}

	// Create a reader from the response body
	reader := resp.Body

	image, err := vision.NewImageFromReader(reader)
	if err != nil {
		return err
	}
	annotations, err := client.DetectTexts(ctx, image, nil, 10)
	if err != nil {
		return err
	}

	if len(annotations) == 0 {
		log.Fatalf("No text found in image: %s", imageUrl)
	}

	// Create a CSV writer
	csvFile, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer csvFile.Close()

	csvWriter := csv.NewWriter(csvFile)
	err = csvWriter.Write([]string{"Text", "X", "Y"}) // Write header row
	if err != nil {
		return err
	}

	isFirstLine := true
	for _, annotation := range annotations {
		// Extract text and bounding polygon
		text := annotation.Description
		vertices := annotation.BoundingPoly.Vertices

		// Calculate and format x,y coordinates
		var xCoord, yCoord int32
		for _, vertex := range vertices {
			if vertex.X > xCoord {
				xCoord = vertex.X
			}
			if vertex.Y > yCoord {
				yCoord = vertex.Y
			}
		}

		// Append text with coordinates to the buffer
		if !isFirstLine {
			err = csvWriter.Write([]string{text, fmt.Sprintf("%d", xCoord), fmt.Sprintf("%d", yCoord)})
			if err != nil {
				return err
			}
		}
		isFirstLine = false
	}

	csvWriter.Flush() // Ensure all data is written

	log.Printf("Successfully saved to %s\n", outputFile)

	return nil
}
