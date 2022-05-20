package starshipkepler

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type PersistentData struct {
	Highscore int
}

func ReadPersistentData() PersistentData {
	persistent := PersistentData{}

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

func WritePersistentData(data PersistentData) {
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
