package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ResponseDictionary converted by https://mholt.github.io/json-to-go/
type ResponseDictionary []struct {
	Word       string      `json:"word"`
	Phonetic   string      `json:"phonetic"`
	Phonetics  []Phonetics `json:"phonetics"`
	Meanings   []Meanings  `json:"meanings"`
	License    License     `json:"license"`
	SourceUrls []string    `json:"sourceUrls"`
}
type License struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}
type Phonetics struct {
	Text      string  `json:"text"`
	Audio     string  `json:"audio"`
	SourceURL string  `json:"sourceUrl,omitempty"`
	License   License `json:"license,omitempty"`
}
type Definitions struct {
	Definition string        `json:"definition"`
	Synonyms   []interface{} `json:"synonyms"`
	Antonyms   []interface{} `json:"antonyms"`
	Example    string        `json:"example,omitempty"`
}
type Meanings struct {
	PartOfSpeech string        `json:"partOfSpeech"`
	Definitions  []Definitions `json:"definitions"`
	Synonyms     []interface{} `json:"synonyms"`
	Antonyms     []interface{} `json:"antonyms"`
}

type Word []string

//go:embed word.json
var wordJson []byte

var collection *mongo.Collection
var ctx = context.TODO()

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	mongodbURI := os.Getenv("MONGODB_URI")
	clientOptions := options.Client().ApplyURI(mongodbURI)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	collection = client.Database("vocabulary").Collection("words")
}

type WordModel struct {
	ID        primitive.ObjectID `bson:"_id"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
	Text      string             `bson:"text"`
}

func main() {
	app := &cli.App{
		Name: "vocabulary",
		Usage: "get a word today",
		Action: func(c *cli.Context) error {

			wordToday, err := getWordFromFile()
			if err != nil {
				log.Fatal(err)
			}

			responseData, err := translateWord(wordToday)
			if err != nil {
				log.Fatal(err)
			}

			printMeaning(responseData)
			return nil
		},
		Commands: []*cli.Command{
			{
				Name: "add",
				Aliases: []string{"a"},
				Usage:   "add a word to the list",
				Action: func(c *cli.Context) error {
					str := c.Args().First()
					if str == "" {
						return errors.New("cannot add an empty word")
					}

					word := &WordModel{
						ID:        primitive.NewObjectID(),
						CreatedAt: time.Now(),
						UpdatedAt: time.Now(),
						Text:      str,
					}

					return saveWord(word)
				},
			},
			{
				Name: "translate",
				Aliases: []string{"t"},
				Usage:   "translate a word",
				Action: func(c *cli.Context) error {
					str := c.Args().First()
					if str == "" {
						return errors.New("cannot add an empty word")
					}

					responseData, err := translateWord(str)
					if err != nil {
						log.Fatal(err)
					}

					printMeaning(responseData)
					return nil
				},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

// getWordFromFile get a word from the JSON file
func getWordFromFile() (w string, e error) {
	var word Word
	err := json.Unmarshal(wordJson, &word)
	if err != nil {
		return "", err
	}

	words, err := getWords()
	for _, v := range words {
		word = append(word, v.Text)
	}

	rand.Seed(time.Now().UnixNano())
	min := 0
	max := len(word)
	randomNumber := rand.Intn(max - min + 1) + min
	wordToday := word[randomNumber]
	return wordToday, nil
}

func translateWord(word string) ([]byte, error){
	URIBuilt := fmt.Sprintf("https://api.dictionaryapi.dev/api/v2/entries/en/%v", word)
	response, err := http.Get(URIBuilt)

	if err != nil {
		return []byte(""), err
	}

	responseData, err := ioutil.ReadAll(response.Body)
	return responseData, nil
}

func printMeaning(responseData []byte) {
	var responseDictionary ResponseDictionary
	err := json.Unmarshal(responseData, &responseDictionary)
	if err != nil {
		fmt.Println("Error load meaning")
	}
	color.Set(color.FgYellow)
	fmt.Println("Word:",strings.ToTitle(strings.ToLower(responseDictionary[0].Word)))
	color.Unset()
	fmt.Println("Meaning:", responseDictionary[0].Meanings[0].Definitions[0].Definition)

	if responseDictionary[0].Meanings[0].Definitions[0].Example  != "" {
		fmt.Println("Example:", responseDictionary[0].Meanings[0].Definitions[0].Example)
	}
}

func saveWord(word *WordModel) error{
	_, err := collection.InsertOne(ctx, word)
	return err
}

func getWords() ([]*WordModel, error){
	// A slice of words for storing the decoded documents
	var words []*WordModel
	cur, err := collection.Find(ctx, bson.D{{}})
	if err != nil {
		return words, err
	}

	// Iterate through the cursor and decode each document one at a time
	for cur.Next(ctx) {
		var w WordModel
		err := cur.Decode(&w)
		if err != nil {
			return words, err
		}

		words = append(words, &w)
	}

	if err := cur.Err(); err != nil {
		return words, err
	}

	// once exhausted, close the cursor
	err = cur.Close(ctx)
	if err != nil {
		return nil, err
	}

	if len(words) == 0 {
		return words, mongo.ErrNoDocuments
	}

	return words, nil
}