package main

import (
	"encoding/csv"
	"io"
	"log"
	"os"
)

const countrycsv = "generate/country.csv"
const zonecsv = "generate/zone.csv"

func main() {

	fzone, err := os.Open(zonecsv)
	if err != nil {
		log.Fatal(err)
	}
	defer fzone.Close()

	// fcountries, err := os.Open(countrycsv)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer fcountries.Close()

	zones := ReadZones(fzone)

	zoneW, err := os.Create("cc_to_zones.go")
	if err != nil {
		log.Fatal(err)
	}

	defer zoneW.Close()

	zoneW.WriteString(`package timezonecompanion

var CCToZones = map[string][]string{
`)

	for cc, z := range zones {
		zoneW.WriteString(`"` + cc + `": []string{`)
		for i, v := range z {
			if i != 0 {
				zoneW.WriteString(`, `)
			}

			zoneW.WriteString(`"` + v + `"`)
		}

		zoneW.WriteString(`},
`)
	}

	zoneW.WriteString("}")
}

func ReadZones(f *os.File) map[string][]string {
	result := make(map[string][]string)

	r := csv.NewReader(f)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		cc := record[1]
		zone := record[2]
		result[cc] = append(result[cc], zone)
	}

	return result
}
