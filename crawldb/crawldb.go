package crawldb

import (
	"encoding/json"
	"fmt"
	"github.com/HouzuoGuo/tiedot/db"
	"github.com/s-rah/onionscan/model"
	"log"
	"time"
)

type CrawlDB struct {
	myDB *db.DB
}

func (cdb *CrawlDB) NewDB(dbdir string) {
	db, err := db.OpenDB(dbdir)
	if err != nil {
		panic(err)
	}
	cdb.myDB = db

	//If we have just created this db then it will be empty
	if len(cdb.myDB.AllCols()) == 0 {
		cdb.Initialize()
	}

}

func (cdb *CrawlDB) Initialize() {
	log.Printf("Creating Database Bucket crawls...")
	if err := cdb.myDB.Create("crawls"); err != nil {
		panic(err)
	}

	// Allow searching by the URL
	log.Printf("Indexing URL in crawls...")
	crawls := cdb.myDB.Use("crawls")
	if err := crawls.Index([]string{"URL"}); err != nil {
		panic(err)
	}

	log.Printf("Creating Database Bucket relationships...")
	if err := cdb.myDB.Create("relationships"); err != nil {
		panic(err)
	}

	// Allowing searching by the Identifier String
	log.Printf("Indexing Identifier in relationships...")
	rels := cdb.myDB.Use("relationships")
	if err := rels.Index([]string{"Identifier"}); err != nil {
		panic(err)
	}

	// Allowing searching by the Onion String
	log.Printf("Indexing Identifier in relationships...")
	if err := rels.Index([]string{"Onion"}); err != nil {
		panic(err)
	}

	log.Printf("Database Setup Complete")

}

func (cdb *CrawlDB) InsertCrawlRecord(url string, page *model.Page) (int, error) {
	crawls := cdb.myDB.Use("crawls")
	docID, err := crawls.Insert(map[string]interface{}{
		"URL":       url,
		"Timestamp": time.Now(),
		"Page":      page})
	return docID, err
}

type CrawlRecord struct {
	URL       string
	Timestamp time.Time
	Page      model.Page
}

func (cdb *CrawlDB) GetCrawlRecord(id int) (CrawlRecord, error) {
	crawls := cdb.myDB.Use("crawls")
	readBack, err := crawls.Read(id)
	if err == nil {
		out, err := json.Marshal(readBack)
		if err == nil {
			var crawlRecord CrawlRecord
			json.Unmarshal(out, &crawlRecord)
			return crawlRecord, nil
		}
		return CrawlRecord{}, err
	}
	return CrawlRecord{}, err
}

func (cdb *CrawlDB) HasCrawlRecord(url string, duration time.Duration) (bool, int) {
	var query interface{}
	before := time.Now().Add(duration)

	q := fmt.Sprintf(`{"eq":"%v", "in": ["URL"]}`, url)
	json.Unmarshal([]byte(q), &query)

	queryResult := make(map[int]struct{}) // query result (document IDs) goes into map keys
	crawls := cdb.myDB.Use("crawls")
	if err := db.EvalQuery(query, crawls, &queryResult); err != nil {
		panic(err)
	}

	for id := range queryResult {
		// To get query result document, simply read it
		readBack, err := crawls.Read(id)
		if err == nil {
			out, err := json.Marshal(readBack)
			if err == nil {
				var crawlRecord CrawlRecord
				json.Unmarshal(out, &crawlRecord)

				if crawlRecord.Timestamp.After(before) {
					return true, id
				}
			}
		}

	}

	return false, 0
}

type Relationship struct {
	ID         int
	Onion      string
	From       string
	Type       string
	Identifier string
	FirstSeen  time.Time
	LastSeen   time.Time
}

func (cdb *CrawlDB) InsertRelationship(onion string, from string, identiferType string, identifier string) (int, error) {

	rels, err := cdb.GetRelationshipsWithOnion(onion)

	// If we have seen this before, we will update rather than adding a
	// new relationship
	if err == nil {
		for _, rel := range rels {
			if rel.From == from && rel.Identifier == identifier && rel.Type == identiferType {
				// Update the Relationships
				log.Printf("Updating %s --- %s ---> %s (%s)", onion, from, identifier, identiferType)
				relationships := cdb.myDB.Use("relationships")
				err := relationships.Update(rel.ID, map[string]interface{}{
					"Onion":      onion,
					"From":       from,
					"Type":       identiferType,
					"Identifier": identifier,
					"FirstSeen":  rel.FirstSeen,
					"LastSeen":   time.Now()})
				return rel.ID, err
			}
		}
	}

	// Otherwise Insert New
	log.Printf("Inserting %s --- %s ---> %s (%s)", onion, from, identifier, identiferType)
	relationships := cdb.myDB.Use("relationships")
	docID, err := relationships.Insert(map[string]interface{}{
		"Onion":      onion,
		"From":       from,
		"Type":       identiferType,
		"Identifier": identifier,
		"FirstSeen":  time.Now(),
		"LastSeen":   time.Now()})
	return docID, err
}

func (cdb *CrawlDB) GetRelationshipsWithOnion(onion string) ([]Relationship, error) {
	var query interface{}

	q := fmt.Sprintf(`{"eq":"%v", "in": ["Onion"]}`, onion)
	json.Unmarshal([]byte(q), &query)

	queryResult := make(map[int]struct{}) // query result (document IDs) goes into map keys
	relationships := cdb.myDB.Use("relationships")
	if err := db.EvalQuery(query, relationships, &queryResult); err != nil {
		return nil, err
	}

	rels := make([]Relationship, 0)
	for id := range queryResult {
		// To get query result document, simply read it
		readBack, err := relationships.Read(id)
		if err == nil {
			out, err := json.Marshal(readBack)
			if err == nil {
				var relationship Relationship
				json.Unmarshal(out, &relationship)
				rels = append(rels, relationship)
			}
		}
	}
	return rels, nil
}

// GetUserRelationshipFromOnion reconstructs a user relationship from a given
// identifier. fromonion is used as a filter to ensure that only user relationships
// from a given onion are reconstructed.
func (cdb *CrawlDB) GetUserRelationshipFromOnion(identifier string, fromonion string) (map[string]Relationship, error) {
	results, err := cdb.GetRelationshipsWithOnion(identifier)

	if err != nil {
		return nil, err
	}

	relationships := make(map[string]Relationship)
	for _, result := range results {
		if result.From == fromonion {
			relationships[result.Type] = result
		}
	}
	return relationships, nil
}

func (cdb *CrawlDB) GetRelationshipsWithIdentifier(identifier string) ([]Relationship, error) {
	var query interface{}

	q := fmt.Sprintf(`{"eq":"%v", "in": ["Identifier"]}`, identifier)
	json.Unmarshal([]byte(q), &query)

	queryResult := make(map[int]struct{}) // query result (document IDs) goes into map keys
	relationships := cdb.myDB.Use("relationships")
	if err := db.EvalQuery(query, relationships, &queryResult); err != nil {
		return nil, err
	}

	rels := make([]Relationship, 0)
	for id := range queryResult {
		// To get query result document, simply read it
		readBack, err := relationships.Read(id)
		if err == nil {
			out, err := json.Marshal(readBack)
			if err == nil {
				var relationship Relationship
				json.Unmarshal(out, &relationship)
				relationship.ID = id
				rels = append(rels, relationship)
			}
		}
	}
	return rels, nil
}
