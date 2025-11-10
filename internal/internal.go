package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/syslog"
	"regexp"
	"strings"
	"time"

	"github.com/sil-org/personnel-sync/v6/alert"
)

const (
	DefaultConfigFile             = "./config.json"
	DefaultVerbosity              = 5
	DestinationTypeGoogleContacts = "GoogleContacts"
	DestinationTypeGoogleGroups   = "GoogleGroups"
	DestinationTypeGoogleSheets   = "GoogleSheets"
	DestinationTypeGoogleUsers    = "GoogleUsers"
	DestinationTypeRestAPI        = "RestAPI"
	DestinationTypeWebHelpDesk    = "WebHelpDesk"
	SourceTypeGoogleSheets        = "GoogleSheets"
	SourceTypeRestAPI             = "RestAPI"
)

// RemapToDestinationAttributes returns a slice of Person instances that each have
// only the desired attributes based on the destination attribute keys.
// If a required attribute is missing for a Person, then their disableChanges
// value is set to true.
func RemapToDestinationAttributes(logger *log.Logger, sourcePersons []Person, attributeMap []AttributeMap) ([]Person, error) {
	var peopleForDestination []Person

	for _, person := range sourcePersons {
		attrs := map[string]string{}

		// Build attrs with only attributes from destination map, disable changes on person missing a required attribute
		disableChanges := false
		for _, attrMap := range attributeMap {
			if value, ok := person.Attributes[attrMap.Source]; ok {
				attrs[attrMap.Destination] = value
			} else if attrMap.Required {
				jsonAttrs, _ := json.Marshal(attrs)
				logger.Printf("user missing attribute %s. Rest of data: %s", attrMap.Source, jsonAttrs)
				disableChanges = true
			}
		}

		peopleForDestination = append(peopleForDestination, Person{
			CompareValue:   person.CompareValue,
			Attributes:     attrs,
			DisableChanges: disableChanges,
		})

	}

	return peopleForDestination, nil
}

// getPersonFromList returns the person if found in peopleList otherwise an empty Person{}
func getPersonFromList(compareValue string, peopleList []Person) Person {
	lowerCompareValue := strings.ToLower(compareValue)

	for _, person := range peopleList {
		if strings.ToLower(person.CompareValue) == lowerCompareValue {
			return person
		}
	}

	return Person{}
}

func personAttributesAreEqual(logger *log.Logger, sp, dp Person, config Config) bool {
	caseSensitivityList := getCaseSensitivitySourceAttributeList(config.AttributeMap)
	equal := true
	for key, val := range sp.Attributes {
		if !stringsAreEqual(val, dp.Attributes[key], caseSensitivityList[key]) {
			if config.Runtime.Verbosity >= VerbosityMedium {
				logger.Printf(`User: "%s", "%s" not equal, CaseSensitive: "%t", Source: "%s", Dest: "%s"`+"\n",
					sp.CompareValue, key, caseSensitivityList[key], val, dp.Attributes[key])
				equal = false
			} else {
				logger.Printf(`User: "%s" not equal`+"\n", key)
				return false
			}
		}
	}

	return equal
}

func stringsAreEqual(val1, val2 string, caseSensitive bool) bool {
	if caseSensitive {
		return val1 == val2
	}

	return strings.ToLower(val1) == strings.ToLower(val2)
}

func getCaseSensitivitySourceAttributeList(attributeMap []AttributeMap) map[string]bool {
	results := map[string]bool{}

	for _, attrMap := range attributeMap {
		results[attrMap.Destination] = attrMap.CaseSensitive
	}

	return results
}

// GenerateChangeSet builds the three slice attributes of a ChangeSet (Create, Update and Delete) based on whether they
// are in the slice of destination Person instances.
//
// It skips all source Person instances that have DisableChanges set to true
func GenerateChangeSet(logger *log.Logger, sourcePeople, destinationPeople []Person, config Config) ChangeSet {
	var changeSet ChangeSet

	// Find users who need to be created or updated
	for _, sp := range sourcePeople {
		// If user was missing a required attribute, don't change their record
		if sp.DisableChanges {
			continue
		}

		sp := processExpressions(logger, config, sp)

		destinationPerson := getPersonFromList(sp.CompareValue, destinationPeople)
		if destinationPerson.CompareValue == "" {
			changeSet.Create = append(changeSet.Create, sp)
			continue
		}

		if !personAttributesAreEqual(logger, sp, destinationPerson, config) {
			sp.ID = destinationPerson.Attributes["id"]
			if destinationPerson.ID != "" { // use ID if it is set to something else
				sp.ID = destinationPerson.ID
			}
			changeSet.Update = append(changeSet.Update, sp)
			continue
		}
	}

	// Find users who need to be deleted
	for _, dp := range destinationPeople {
		sourcePerson := getPersonFromList(dp.CompareValue, sourcePeople)
		if sourcePerson.CompareValue == "" {
			changeSet.Delete = append(changeSet.Delete, dp)
		}
	}

	return changeSet
}

func processExpressions(logger *log.Logger, config Config, person Person) Person {
	for _, attr := range config.AttributeMap {
		if attr.Expression == "" {
			continue
		}

		attrName := attr.Destination // the remap to destination attributes has happened already

		attrValue, _ := person.Attributes[attrName]

		re, err := regexp.Compile(attr.Expression)
		if err != nil {
			msg := fmt.Errorf("invalid regular expression (%q) on attribute %s",
				attr.Expression, attrName)
			logger.Println(msg)
			alert.New(config.Alert, msg)
			continue
		}

		n := re.ReplaceAllString(attrValue, attr.Replace)
		person.Attributes[attrName] = n
	}
	return person
}

// RunSyncSet calls a number of functions to do the following ...
//   - it gets the list of people from the source
//   - it remaps their attributes to match the keys used in the destination
//   - it gets the list of people from the destination
//   - it generates the lists of people to change, update and delete
//   - if dryRun is true, it prints those lists, otherwise it makes the associated changes
func RunSyncSet(logger *log.Logger, source Source, destination Destination, config Config) error {
	sourcePeople, err := source.ListUsers(GetSourceAttributes(config.AttributeMap))
	if err != nil {
		return err
	}
	if len(sourcePeople) == 0 {
		return errors.New("no people found in source")
	}
	logger.Printf("    Found %v people in source", len(sourcePeople))

	// remap source people to destination attributes for comparison
	sourcePeople, err = RemapToDestinationAttributes(logger, sourcePeople, config.AttributeMap)
	if err != nil {
		return err
	}

	destinationPeople, err := destination.ListUsers(GetDestinationAttributes(config.AttributeMap))
	if err != nil {
		return err
	}
	logger.Printf("    Found %v people in destination", len(destinationPeople))

	changeSet := GenerateChangeSet(logger, sourcePeople, destinationPeople, config)

	logger.Printf("ChangeSet Plans: Create %d, Update %d, Delete %d\n",
		len(changeSet.Create), len(changeSet.Update), len(changeSet.Delete))

	// If in DryRun mode only print out ChangeSet plans and return mocked change results based on plans
	if config.Runtime.DryRunMode {
		logger.Println("Dry run mode enabled. Change set details follow:")
		printChangeSet(logger, changeSet)
		return nil
	}

	// Create a channel to pass activity logs for printing
	eventLog := make(chan EventLogItem, 50)
	go processEventLog(logger, config.Alert, eventLog)

	results := destination.ApplyChangeSet(changeSet, eventLog)

	for range 100 {
		time.Sleep(time.Millisecond * 10)
		if len(eventLog) == 0 {
			break
		}
	}
	close(eventLog)

	logger.Printf("Sync results: %v users added, %v users updated, %v users removed\n",
		results.Created, results.Updated, results.Deleted)

	return nil
}

func GetSourceAttributes(attrMap []AttributeMap) []string {
	var keys []string
	for _, attr := range attrMap {
		if attr.Source != "" {
			keys = append(keys, attr.Source)
		}
	}

	return keys
}

func GetDestinationAttributes(attrMap []AttributeMap) []string {
	var keys []string
	for _, attr := range attrMap {
		if attr.Destination != "" {
			keys = append(keys, attr.Destination)
		}
	}

	return keys
}

func processEventLog(logger *log.Logger, config alert.Config, eventLog <-chan EventLogItem) {
	for msg := range eventLog {
		logger.Println(msg)
		if msg.Level == syslog.LOG_ALERT || msg.Level == syslog.LOG_EMERG {
			alert.New(config, msg)
		}
	}
}

func printChangeSet(logger *log.Logger, changeSet ChangeSet) {
	logger.Printf("Users to be created: %d ...", len(changeSet.Create))
	for i, user := range changeSet.Create {
		logger.Printf("  create %v) %s", i+1, user.CompareValue)
	}

	logger.Printf("Users to be updated: %d ...", len(changeSet.Update))
	for i, user := range changeSet.Update {
		logger.Printf("  update %v) %s", i+1, user.CompareValue)
	}

	logger.Printf("Users to be deleted: %d ...", len(changeSet.Delete))
	for i, user := range changeSet.Delete {
		logger.Printf("  delete %v) %s", i+1, user.CompareValue)
	}
}

type EmptyDestination struct{}

func (e *EmptyDestination) ForSet(syncSetJson json.RawMessage) error {
	return nil
}

func (e *EmptyDestination) ListUsers(desiredAttrs []string) ([]Person, error) {
	return []Person{}, nil
}

func (e *EmptyDestination) ApplyChangeSet(changes ChangeSet, eventLog chan<- EventLogItem) ChangeResults {
	return ChangeResults{}
}

type EmptySource struct{}

func (e *EmptySource) ForSet(syncSetJson json.RawMessage) error {
	return nil
}

func (e *EmptySource) ListUsers(desiredAttrs []string) ([]Person, error) {
	return []Person{}, nil
}

// NewBatchTimer returns a new BatchTimer with the startTime set to the current time and the endTime set to
// secondsPerBatch from now
func NewBatchTimer(batchSize, secondsPerBatch int) BatchTimer {
	b := BatchTimer{}
	b.Init(batchSize, secondsPerBatch)
	return b
}

// BatchTimer is intended as a time limited batch enforcer. To create one, call its Init method.
// Then, to use it call its WaitOnBatch method after every call to the associated go routine
type BatchTimer struct {
	startTime       time.Time
	endTime         time.Time
	Counter         int
	SecondsPerBatch int
	BatchSize       int
}

// Init sets the startTime to the current time, sets the endTime based on secondsPerBatch into the future
func (b *BatchTimer) Init(batchSize, secondsPerBatch int) {
	b.startTime = time.Now()
	b.setEndTime()
	b.SecondsPerBatch = secondsPerBatch
	b.BatchSize = batchSize
	b.Counter = 0
}

func (b *BatchTimer) setEndTime() {
	var emptyTime time.Time
	if b.startTime == emptyTime {
		b.startTime = time.Now()
	}
	b.endTime = b.startTime.Add(time.Second * time.Duration(b.SecondsPerBatch))
}

// WaitOnBatch increments the Counter and then if fewer than BatchSize have been dealt with, just returns without doing
// anything Otherwise, sleeps until the batch time has expired (i.e. current time is past endTime). If this last process
// occurs, then it ends by resetting the batch's times and counter.
func (b *BatchTimer) WaitOnBatch() {
	b.Counter++
	if b.Counter < b.BatchSize {
		return
	}

	for {
		currTime := time.Now()
		if currTime.After(b.endTime) {
			break
		}
		time.Sleep(time.Second)
	}
	b.Init(b.BatchSize, b.SecondsPerBatch)
}
