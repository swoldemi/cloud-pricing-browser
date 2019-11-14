package cloudability

import (
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"os"
	"time"
)

var ignoredKeys = []string{
	"tag_user_Name",
}

type Tag struct {
	gorm.Model
	Key      string `json:"vendorKey"`
	Value    string `json:"vendorValue"`
	ResultID uint
}

type UniqueTag struct {
	gorm.Model
	Key     string  `json:"key"`
	Value   string  `json:"value"`
	Count   int     `json:"count"`
	Cost    float64 `json:"cost"`
	Hourly  float64 `json:"hourly"`
	Monthly float64 `json:"monthly"`
}

type Result struct {
	gorm.Model
	Service            string    `json:"service"`
	Name               string    `json:"name"`
	ResourceIdentifier string    `json:"resourceIdentifier" gorm:"unique"`
	VendorAccountId    string    `json:"vendorAccountId"`
	Tags               []Tag     `json:"tags" gorm:"foreignkey:ResultID" gorm:"auto_preload"`
	Provider           string    `json:"provider"`
	Region             string    `json:"region"`
	OS                 string    `json:"os"`
	NodeType           string    `json:"nodeType"`
	EffectiveHourly    float64   `json:"effectiveHourly"`
	TotalSpend         float64   `json:"totalSpend"`
	LastSeen           time.Time `json:"lastSeen"`
	HoursRunning       int       `jons:"hoursRunning"`
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func PostgresConnect() *gorm.DB {
	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")
	dbName := os.Getenv("DB_NAME")
	dbHost := os.Getenv("DB_HOST")

	db, err := gorm.Open(
		"postgres",
		fmt.Sprintf("host=%s port=5432 user=%s dbname=%s password=%s sslmode=disable", dbHost, dbUser, dbName, dbPass),
	)

	if err != nil {
		fmt.Println(err)
		panic("failed to connect database")
	}
	defer db.Close()

	return db
}

func GetTagKeysAndValues() []UniqueTag {
	var tags []UniqueTag
	// db, err := gorm.Open("sqlite3", "test.db")
	db := PostgresConnect()
	db.Find(&tags)
	return tags
}

func GetInstances() []Result {
	var instances []Result
	db := PostgresConnect()
	db.Joins("left join tags on results.id = tags.result_id").Preload("Tags").Find(&instances)

	return instances
}

func PopulateUniqueTags(results []Result) []Result {
	tagInfo := make(map[string]map[string]UniqueTag)

	for _, result := range results {
		for _, tag := range result.Tags {
			fmt.Printf("%s  %s\n\n", tag.Key, tag.Value)
			if _, ok := tagInfo[tag.Key]; ok {
				if _, ok := tagInfo[tag.Key][tag.Value]; ok {
					var tmpStruct = tagInfo[tag.Key][tag.Value]
					tmpStruct.Cost = tmpStruct.Cost + result.TotalSpend
					tmpStruct.Count = tmpStruct.Count + 1
					tmpStruct.Hourly = tmpStruct.Hourly + result.EffectiveHourly
					tmpStruct.Monthly = tmpStruct.Monthly + (result.EffectiveHourly * 24 * 30)
					tagInfo[tag.Key][tag.Value] = tmpStruct
				} else {
					tagInfo[tag.Key][tag.Value] = UniqueTag{
						Key:     tag.Key,
						Value:   tag.Value,
						Count:   1,
						Cost:    result.TotalSpend,
						Hourly:  result.EffectiveHourly,
						Monthly: (result.EffectiveHourly * 24 * 30),
					}
				}
			} else {
				tagInfo[tag.Key] = make(map[string]UniqueTag)
				tagInfo[tag.Key][tag.Value] = UniqueTag{
					Key:     tag.Key,
					Value:   tag.Value,
					Cost:    result.TotalSpend,
					Count:   1,
					Hourly:  result.EffectiveHourly,
					Monthly: (result.EffectiveHourly * 24 * 30),
				}
				tagInfo[tag.Key]["none"] = UniqueTag{
					Key:     tag.Key,
					Value:   "none",
					Count:   0,
					Cost:    0,
					Hourly:  0,
					Monthly: 0,
				}

			}
		}
	}

	db := PostgresConnect()

	for _, result := range results {
		for tag, _ := range tagInfo {
			if stringArrayDoesNotContain(GetKeys(result.Tags), tag) {
				var tmpStruct = tagInfo[tag]["none"]
				tmpStruct.Count = tmpStruct.Count + 1
				tmpStruct.Cost = tmpStruct.Cost + result.TotalSpend
				tmpStruct.Hourly = tmpStruct.Hourly + result.EffectiveHourly
				tmpStruct.Monthly = tmpStruct.Monthly + (result.EffectiveHourly * 24 * 30)
				tagInfo[tag]["none"] = tmpStruct
			}
		}
	}

	for _, a := range tagInfo {
		for _, b := range a {
			var uniqueTag UniqueTag
			if stringArrayDoesNotContain(ignoredKeys, b.Key) {
				db.Where(UniqueTag{Key: b.Key, Value: b.Value}).Assign(b).FirstOrCreate(&uniqueTag)
			}
		}
	}
	return results
}

func GetKeys(tags []Tag) []string {
	var retVal []string
	for _, t := range tags {
		retVal = append(retVal, t.Key)
	}

	return retVal
}

func stringArrayDoesNotContain(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return false
		}
	}
	return true
}

func WriteResults(results []Result) []Result {
	db := PostgresConnect()

	// Migrate the schema
	db.AutoMigrate(&Tag{})
	db.AutoMigrate(&Result{})
	db.AutoMigrate(&UniqueTag{})
	// Create
	for _, result := range results {
		var thisResult Result
		db.Where(Result{ResourceIdentifier: result.ResourceIdentifier}).Assign(result).FirstOrCreate(&thisResult)
		// db.Create(&result)
	}

	return results
}