package starshipkepler

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type LocalData struct {
	Scoreboard []ScoreEntry
}

func ReadLocalData() LocalData {
	persistent := LocalData{}

	data, err := ioutil.ReadFile("./gamedata.yml")
	if err != nil {
		data = []byte{}
	}

	err = yaml.Unmarshal([]byte(data), &persistent)
	if err != nil {
		fmt.Printf("[Boot] error loading persistent data")
	}

	return persistent
}

func (data *LocalData) WriteToFile() {
	yml, err := yaml.Marshal(&data)
	if err != nil {
		log.Fatalf("[persistence] error: %v", err)
	}

	f, err := os.Create("./gamedata.yml")
	if err != nil {
		log.Fatalf("[persistence] error writing data: %v", err)
	}
	defer f.Close()
	f.Write(yml)
}
