package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
)

type ExtraRelationship struct {
	Name  string `json:"name"`
	Regex string `json:"regex"`
	Rollup                 bool                `json:"rollup"`
}

type Relationship struct {
	Name                   string              `json:"name"`
	TriggerIdentifierRegex string              `json:"triggeridentifierregex"`
	ExtraRelationships     []ExtraRelationship `json:"extrarelationships"`
}

type CrawlConfig struct {
	Onion         string         `json:"onion"`
	Base          string         `json:"base"`
	Exclude       []string       `json:"exclude"`
	Relationships []Relationship `json:"relationships"`
}

func (cc *CrawlConfig) GetRelationship(name string) (Relationship, error) {
	for _, relationship := range cc.Relationships {
		if relationship.Name == name {
			return relationship, nil
		}
	}
	return Relationship{}, errors.New(fmt.Sprintf(`Could not find Relationship "%s"`, name))
}

func LoadCrawlConfig(filename string) (CrawlConfig, error) {
	dat, err := ioutil.ReadFile(filename)
	if err != nil {
		return CrawlConfig{}, err
	}
	res := CrawlConfig{}
	err = json.Unmarshal(dat, &res)
	return res, err
}
