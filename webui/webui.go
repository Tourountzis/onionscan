package webui

import (
	"errors"
	"github.com/s-rah/onionscan/config"
	"github.com/s-rah/onionscan/crawldb"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type WebUI struct {
	osc  *config.OnionScanConfig
	Done chan bool
}

type Content struct {
	SearchTerm string
	Error      string
	Tables     []Table
}

type Row struct {
	Fields []string
}

type Table struct {
	Title   string
	SearchTerm string
	Heading []string
	Rows    []Row
	Rollups  []int
	RollupCounts map[string]int
}

// GetUserDefinedRow returns, from an initial relationship, a complete user
// defined relationship row - in the order it is defined in the crawl config.
func (wui *WebUI) GetUserDefinedTable(rel crawldb.Relationship) (Table, error) {
	log.Printf("Loading User Defined Relationship %s", rel.From)
	config, ok := wui.osc.CrawlConfigs[rel.From]
	if ok {
		var table Table
		crName := strings.SplitN(rel.Type, "/", 2)
		if len(crName) == 2 {
			table.Title = crName[0]
			cr, err := config.GetRelationship(crName[0])
			if err == nil {
				for i, er := range cr.ExtraRelationships {
					table.Heading = append(table.Heading, er.Name)
					if er.Rollup {
					        table.Rollups = append(table.Rollups, i)
					}
				}
				table.Heading = append(table.Heading, "Onion")
				log.Printf("Returning User Table Relationship %v", table)
				return table, nil
			}
		}
	}
	log.Printf("Could not make Table")
	return Table{}, errors.New("Invalid Table")
}

// GetUserDefinedRow returns, from an initial relationship, a complete user
// defined relationship row - in the order it is defined in the crawl config.
func (wui *WebUI) GetUserDefinedRow(rel crawldb.Relationship) (string, []string) {
	log.Printf("Loading User Defined Relationship %s", rel.From)
	config, ok := wui.osc.CrawlConfigs[rel.From]

	if ok {
		userrel, err := wui.osc.Database.GetUserRelationshipFromOnion(rel.Onion, rel.From)

		if err == nil {
			// We can now construct the user
			// relationship in the right order.
			crName := strings.SplitN(rel.Type, "/", 2)
			if len(crName) == 2 {
				cr, err := config.GetRelationship(crName[0])
				row := make([]string, 0)
				if err == nil {
					for _, er := range cr.ExtraRelationships {
						log.Printf("Field Value: %v", userrel[crName[0]+"/"+er.Name].Identifier)
						row = append(row, userrel[crName[0]+"/"+er.Name].Identifier)
					}
					row = append(row, rel.From)
					log.Printf("Returning User Row Relationship %s %v %s", crName[0], row, rel.Onion)
					return crName[0], row
				}

			} else {
				log.Printf("Could not derive config relationship from type %s", rel.Type)
			}
		}
	}
	log.Printf("Invalid Row")
	return "", []string{}
}

func (wui *WebUI) Index(w http.ResponseWriter, r *http.Request) {

	search := r.URL.Query().Get("search")
	var content Content
	if search != "" {
		content.SearchTerm = search

		var results []crawldb.Relationship
		//var err error
		tables := make(map[string]Table)

		if strings.HasSuffix(search, ".onion") {
			results, _ = wui.osc.Database.GetRelationshipsWithOnion(search)
		} else {
			results, _ = wui.osc.Database.GetRelationshipsWithIdentifier(search)
		}

		for _, rel := range results {
			if strings.HasSuffix(rel.Onion, ".onion") && rel.Type != "database-id" &&  rel.Type != "user-relationship"{
				table, ok := tables[rel.Type]
				if !ok {
					var newTable Table
					newTable.Title = rel.Type
					newTable.Heading = []string{"Identifier", "Onion"}
					tables[rel.Type] = newTable
					table = newTable
				}
				table.Rows = append(table.Rows, Row{Fields: []string{rel.Identifier, rel.Onion}})
				tables[rel.Type] = table
			} else if strings.HasSuffix(rel.From, ".onion") {
				tableName, row := wui.GetUserDefinedRow(rel)

				if len(row) > 0 {
					table, exists := tables[tableName]
					if !exists {
						newTable, err := wui.GetUserDefinedTable(rel)
						if err == nil {
							tables[tableName] = newTable
							table = newTable
						}
					}
					table.Rows = append(table.Rows, Row{Fields: row})
					tables[tableName] = table
				}
			} else if rel.Type == "user-relationship" {  
			        // Hack userrel
			        userrel := rel
			        userrel.Onion = rel.Identifier
			        userrel.From = rel.Onion
			        userrel.Type = rel.From+"/parent"
			        tableName, row := wui.GetUserDefinedRow(userrel)

			        if len(row) > 0 {
				        table, exists := tables[tableName]
				        if !exists {
					        newTable, err := wui.GetUserDefinedTable(userrel)
					        if err == nil {
						        tables[tableName] = newTable
						        table = newTable
					        }
				        }
				        table.Rows = append(table.Rows, Row{Fields: row})
				        tables[tableName] = table
			        }			        
			        //}
			}

		}

		// We now have a bunch of tables, keyed by type.
		for k, v := range tables {
			log.Printf("Adding Table %s %v", k, v)
			
			rollups := make(map[string]int)
			for _,c := range v.Rollups {
			        for _,rows := range v.Rows {
                                        rollups[rows.Fields[c]]++
			        }
			}
			v.RollupCounts = rollups
			v.SearchTerm = search
			content.Tables = append(content.Tables, v)
		}

	}

	var templates = template.Must(template.ParseFiles("templates/index.html"))
	templates.ExecuteTemplate(w, "index.html", content)
}

func (wui *WebUI) Listen(osc *config.OnionScanConfig, port int) {
	wui.osc = osc
	http.HandleFunc("/", wui.Index)

	fs := http.FileServer(http.Dir("./templates/style"))
	http.Handle("/style/", http.StripPrefix("/style/", fs))

	fs = http.FileServer(http.Dir("./templates/scripts"))
	http.Handle("/scripts/", http.StripPrefix("/scripts/", fs))

	portstr := strconv.Itoa(port)
	http.ListenAndServe("127.0.0.1:"+portstr, nil)
}
