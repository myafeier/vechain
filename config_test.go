package vechain

import (
	"os"

	"github.com/gf-third/yaml"
)

var config VechainConfig

func init() {
	fs, err := os.Open("./config.yml")
	if err != nil {
		panic(err)
	}
	defer fs.Close()
	err = yaml.NewDecoder(fs).Decode(&config)
	if err != nil {
		panic(err)
	}
	if err != nil {
		panic(err)
	}
}
