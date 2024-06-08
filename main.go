package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	vision "cloud.google.com/go/vision/apiv1"
	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
	"golang.org/x/net/context"
)

// Bot parameters
var (
	RemoveCommands = flag.Bool("rmcmd", true, "Remove all commands after shutdowning or not")
)

var s *discordgo.Session

func init() { flag.Parse() }
func init() { envLoad() }

func init() {
	var err error
	BotToken := os.Getenv("DISCORD_TOKEN")
	s, err = discordgo.New("Bot " + BotToken)
	if err != nil {
		log.Fatalf("Invalid bot parameters: %v", err)
	}
}

var (
	commands = []*discordgo.ApplicationCommand{
		{
			Name: "basic-command",
			// All commands and options must have a description
			// Commands/options without description will fail the registration
			// of the command.
			Description: "Basic command",
		},
		{
			Name:        "ocr",
			Description: "Read Game Result Image with OCR",
		},
	}

	commandHandlers = map[string]func(s *discordgo.Session, i *discordgo.InteractionCreate){
		"basic-command": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Hey there! Congratulations, you just executed your first slash command",
				},
			})
		},
		"ocr": func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			imagepath := "testdata/sdvx_result.jpg"
			outputpath := "output.csv"
			err := detectTextAndSaveToFile(outputpath, imagepath)
			if err != nil {
				log.Fatalf("Failed to read result image: %v", err)
			}
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
			})
		},
	}
)

func init() {
	s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if h, ok := commandHandlers[i.ApplicationCommandData().Name]; ok {
			h(s, i)
		}
	})
}

func envLoad() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading env target")
	}
}

// detectText gets text from the Vision API for an image at the given file path.
func detectTextAndSaveToFile(outputFile string, imagepath string) error {
	ctx := context.Background()

	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		return err
	}

	file, err := os.Open(imagepath)
	if err != nil {
		return err
	}
	defer file.Close()

	image, err := vision.NewImageFromReader(file)
	if err != nil {
		return err
	}
	annotations, err := client.DetectTexts(ctx, image, nil, 10)
	if err != nil {
		return err
	}

	if len(annotations) == 0 {
		log.Fatalf("No text found in image: %s", imagepath)
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

	log.Printf("Text from image %s successfully saved to %s\n", imagepath, outputFile)

	return nil
}

func main() {
	GuildID := os.Getenv("TEST_GUILD")
	s.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		log.Printf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	})
	err := s.Open()
	if err != nil {
		log.Fatalf("Cannot open the session: %v", err)
	}

	log.Println("Adding commands...")
	registeredCommands := make([]*discordgo.ApplicationCommand, len(commands))
	for i, v := range commands {
		cmd, err := s.ApplicationCommandCreate(s.State.User.ID, GuildID, v)
		if err != nil {
			log.Panicf("Cannot create '%v' command: %v", v.Name, err)
		}
		registeredCommands[i] = cmd
	}

	defer s.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	log.Println("Press Ctrl+C to exit")
	<-stop

	if *RemoveCommands {
		log.Println("Removing commands...")
		// // We need to fetch the commands, since deleting requires the command ID.
		// // We are doing this from the returned commands on line 375, because using
		// // this will delete all the commands, which might not be desirable, so we
		// // are deleting only the commands that we added.
		// registeredCommands, err := s.ApplicationCommands(s.State.User.ID, *GuildID)
		// if err != nil {
		// 	log.Fatalf("Could not fetch registered commands: %v", err)
		// }

		for _, v := range registeredCommands {
			err := s.ApplicationCommandDelete(s.State.User.ID, GuildID, v.ID)
			if err != nil {
				log.Panicf("Cannot delete '%v' command: %v", v.Name, err)
			}
		}
	}

	log.Println("Gracefully shutting down.")
}
