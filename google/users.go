package google

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/syslog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sil-org/personnel-sync/v6/internal"

	"golang.org/x/net/context"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/googleapi"
)

type GoogleUsers struct {
	DestinationConfig internal.DestinationConfig
	BatchSize         int
	BatchDelaySeconds int
	GoogleConfig      GoogleConfig
	AdminService      admin.Service
}

func NewGoogleUsersDestination(destinationConfig internal.DestinationConfig) (internal.Destination, error) {
	var googleUsers GoogleUsers
	// Unmarshal ExtraJSON into GoogleConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &googleUsers.GoogleConfig)
	if err != nil {
		return &GoogleUsers{}, err
	}

	// Defaults
	if googleUsers.BatchSize <= 0 {
		googleUsers.BatchSize = DefaultBatchSize
	}
	if googleUsers.BatchDelaySeconds <= 0 {
		googleUsers.BatchDelaySeconds = DefaultBatchDelaySeconds
	}

	googleUsers.DestinationConfig = destinationConfig

	// Initialize AdminService object
	googleUsers.AdminService, err = initGoogleAdminService(
		googleUsers.GoogleConfig.GoogleAuth,
		googleUsers.GoogleConfig.DelegatedAdminEmail,
		admin.AdminDirectoryUserScope,
	)
	if err != nil {
		return &GoogleUsers{}, err
	}

	return &googleUsers, nil
}

func (g *GoogleUsers) ForSet(syncSetJson json.RawMessage) error {
	// sync sets not implemented for this destination
	return nil
}

func extractData(user admin.User) internal.Person {
	attributes := map[string]string{"email": strings.ToLower(user.PrimaryEmail)}

	if found := findFirstMatchingType(user.ExternalIds, "organization"); found != nil {
		setStringFromInterface(found["value"], attributes, "id")
	}

	if found := findFirstMatchingType(user.Locations, "desk"); found != nil {
		setStringFromInterface(found["area"], attributes, "area")
	}

	if found := findFirstMatchingType(user.Organizations, ""); found != nil {
		setStringFromInterface(found["costCenter"], attributes, "costCenter")
		setStringFromInterface(found["department"], attributes, "department")
		setStringFromInterface(found["title"], attributes, "title")
	}

	if found := findFirstMatchingType(user.Relations, "manager"); found != nil {
		setStringFromInterface(found["value"], attributes, "manager")
	}

	if user.Name != nil {
		attributes["familyName"] = user.Name.FamilyName
		attributes["givenName"] = user.Name.GivenName
	}

	for schemaKey, schemaVal := range user.CustomSchemas {
		var schema map[string]string
		_ = json.Unmarshal(schemaVal, &schema)
		for propertyKey, propertyVal := range schema {
			attributes[schemaKey+"."+propertyKey] = propertyVal
		}
	}

	attributes = mergeAttributeMaps(attributes, getPhoneNumbersFromUser(user))

	return internal.Person{CompareValue: user.PrimaryEmail, Attributes: attributes}
}

// getPhoneNumbersFromUser converts phone attributes from an admin.User struct and saves attributes in a string map
func getPhoneNumbersFromUser(user admin.User) map[string]string {
	attributes := map[string]string{}

	phones, ok := user.Phones.([]any)
	if !ok {
		return attributes
	}

	for _, phoneAsInterface := range phones {
		phone := phoneAsInterface.(map[string]any)

		phoneType, ok := phone["type"].(string)
		if !ok {
			continue
		}
		val, ok := phone["value"].(string)
		if !ok {
			continue
		}
		custom, _ := phone["customType"].(string)

		key := phoneKey(phoneType, custom, attributes)
		attributes[key] = val
	}

	return attributes
}

// phoneKey generates a key that includes the phone type and custom type. It recursively searches attributes for
// duplicate keys, incrementing a numeric suffix when necessary to make it unique.
func phoneKey(phoneType, custom string, attributes map[string]string) string {
	key := "phone" + delim + phoneType

	if strings.HasPrefix(phoneType, "custom") && custom != "" {
		key += delim + custom
	}

	_, ok := attributes[key]
	if ok {
		split := strings.SplitN(phoneType, "~", 2)
		n := 0
		if len(split) > 1 {
			n, _ = strconv.Atoi(split[1])
		}

		return phoneKey(fmt.Sprintf("%s~%d", split[0], n+1), custom, attributes)
	}

	return key
}

// findFirstMatchingType iterates through a slice of interfaces until it finds a matching key. The underlying type
// of the given interface must be `[]map[string]any`. If `findType` is empty, the first element in the
// slice is returned.
func findFirstMatchingType(in any, findType string) map[string]any {
	sliceOfInterfaces, ok := in.([]any)
	if !ok {
		return nil
	}
	for _, i := range sliceOfInterfaces {
		if i2 := isMatchingType(i, findType); i2 != nil {
			return i2
		}
	}
	return nil
}

// isMatchingType returns the value of `i`, cast to `map[string]any` if it contains an entry with key 'type'
// and value equal to `findType`. If `findType` is empty, the first element in the slice is returned.
func isMatchingType(i any, findType string) map[string]any {
	if m, ok := i.(map[string]any); ok {
		if findType == "" {
			return m
		}
		if recordType, ok := m["type"].(string); ok && recordType == findType {
			return m
		}
	}
	return nil
}

// setStringFromInterface gets a string from an empty interface, and assigns it to a map
func setStringFromInterface(i any, m map[string]string, key string) {
	if value, ok := i.(string); ok {
		m[key] = value
	}
}

func (g *GoogleUsers) ListUsers(desiredAttrs []string) ([]internal.Person, error) {
	var usersList []*admin.User
	usersListCall := g.AdminService.Users.List()
	usersListCall.Customer("my_customer") // query all domains in this GSuite
	usersListCall.Projection("full")      // include custom fields
	err := usersListCall.Pages(context.TODO(), func(users *admin.Users) error {
		usersList = append(usersList, users.Users...)
		return nil
	})
	if err != nil {
		syncErr := internal.SyncError{
			Message:   fmt.Errorf("unable to get users: %w", err),
			SendAlert: true,
		}
		if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusServiceUnavailable {
			syncErr.SendAlert = false
		}
		return []internal.Person{}, syncErr
	}

	var people []internal.Person
	for _, nextUser := range usersList {
		if nextUser != nil {
			people = append(people, extractData(*nextUser))
		}
	}
	return people, nil
}

func (g *GoogleUsers) ApplyChangeSet(
	changes internal.ChangeSet,
	eventLog chan<- internal.EventLogItem,
) internal.ChangeResults {
	var results internal.ChangeResults
	var wg sync.WaitGroup

	// One minute per batch
	batchTimer := internal.NewBatchTimer(g.BatchSize, g.BatchDelaySeconds)

	if !g.DestinationConfig.DisableUpdate {
		for _, toUpdate := range changes.Update {
			wg.Add(1)
			go g.updateUser(toUpdate, &results.Updated, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	wg.Wait()

	return results
}

func newUserForUpdate(person internal.Person, oldUser admin.User) (admin.User, error) {
	user := admin.User{}
	var err error
	var organization admin.UserOrganization
	isOrgModified := false

	phones := getPhoneNumbersFromUser(oldUser)

	for key, val := range person.Attributes {
		switch beforeDelim(key) {
		case "givenName":
			if user.Name == nil {
				user.Name = &admin.UserName{GivenName: val}
			} else {
				user.Name.GivenName = val
			}

		case "familyName":
			if user.Name == nil {
				user.Name = &admin.UserName{FamilyName: val}
			} else {
				user.Name.FamilyName = val
			}

		case "id":
			user.ExternalIds, err = updateIDs(val, oldUser.ExternalIds)
			if err != nil {
				return admin.User{}, err
			}

		case "area":
			user.Locations, err = updateLocations(val, oldUser.Locations)
			if err != nil {
				return admin.User{}, err
			}

		case "costCenter":
			organization.CostCenter = val
			isOrgModified = true

		case "department":
			organization.Department = val
			isOrgModified = true

		case "title":
			organization.Title = val
			isOrgModified = true

		case "phone":
			phones[key] = val

		case "manager":
			user.Relations, err = updateRelations(val, oldUser.Relations)
			if err != nil {
				return admin.User{}, err
			}

		default:
			keys := strings.SplitN(key, ".", 2)
			if len(keys) < 2 {
				continue
			}

			j, err := json.Marshal(&map[string]string{keys[1]: val})
			if err != nil {
				return admin.User{}, fmt.Errorf("error marshaling custom schema, %s", err)
			}

			user.CustomSchemas = map[string]googleapi.RawMessage{
				keys[0]: j,
			}
		}
	}

	user.Phones, err = attributesToUserPhones(phones)
	if err != nil {
		return admin.User{}, err
	}

	if isOrgModified {
		// NOTICE: this will overwrite any and all existing Organizations
		user.Organizations = []admin.UserOrganization{organization}
	}

	return user, nil
}

func beforeDelim(s string) string {
	split := strings.SplitN(s, delim, 2)
	return split[0]
}

func (g *GoogleUsers) updateUser(
	person internal.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- internal.EventLogItem,
) {
	defer wg.Done()

	email := person.Attributes["email"]

	oldUser, err := g.getUser(person.CompareValue)
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to get old user %s, %s", email, err.Error()),
		}
		return
	}

	newUser, err2 := newUserForUpdate(person, oldUser)
	if err2 != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to prepare update for %s in Users: %s", email, err2.Error()),
		}
		return
	}

	_, err3 := g.AdminService.Users.Update(email, &newUser).Do()
	if err3 != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to update %s in Users: %s", email, err3.Error()),
		}
		return
	}

	eventLog <- internal.EventLogItem{
		Level:   syslog.LOG_INFO,
		Message: "UpdateUser " + email,
	}

	atomic.AddUint64(counter, 1)
}

func (g *GoogleUsers) getUser(email string) (admin.User, error) {
	userCall := g.AdminService.Users.Get(email)
	user, err := userCall.Do()
	if err != nil || user == nil {
		return admin.User{}, err
	}
	return *user, nil
}

func updateIDs(newID string, oldIDs any) ([]admin.UserExternalId, error) {
	IDs := []admin.UserExternalId{{
		Type:  "organization",
		Value: newID,
	}}

	if oldIDs == nil {
		return IDs, nil
	}

	interfaces, ok := oldIDs.([]any)
	if !ok {
		return nil, errors.New("no slice in Google API ExternalIDs")
	}

	for i := range interfaces {
		IDMap, ok := interfaces[i].(map[string]any)
		if !ok {
			return nil, errors.New("unexpected data in Google API ID list")
		}

		thisType, ok := IDMap["type"].(string)
		if !ok {
			return nil, errors.New("unexpected data in Google API ID list entry")
		}

		if thisType == "organization" {
			continue
		}

		value, _ := IDMap["value"].(string)
		customType, _ := IDMap["customType"].(string)
		IDs = append(IDs, admin.UserExternalId{
			Type:       thisType,
			CustomType: customType,
			Value:      value,
		})
	}

	return IDs, nil
}

func updateLocations(newArea string, oldLocations any) ([]admin.UserLocation, error) {
	locations := []admin.UserLocation{{
		Type: "desk",
		Area: newArea,
	}}

	if oldLocations == nil {
		return locations, nil
	}

	interfaces, ok := oldLocations.([]any)
	if !ok {
		return nil, errors.New("no slice in Google API Locations")
	}

	for i := range interfaces {
		locationMap, ok := interfaces[i].(map[string]any)
		if !ok {
			return nil, errors.New("unexpected data in Google API location list")
		}

		thisType, ok := locationMap["type"].(string)
		if !ok {
			return nil, errors.New("unexpected data in Google API location list entry")
		}

		if thisType == "desk" {
			continue
		}

		area, _ := locationMap["area"].(string)
		buildingId, _ := locationMap["buildingId"].(string)
		customType, _ := locationMap["customType"].(string)
		deskCode, _ := locationMap["deskCode"].(string)
		floorName, _ := locationMap["floorName"].(string)
		floorSection, _ := locationMap["floorSection"].(string)
		locations = append(locations, admin.UserLocation{
			Type:         thisType,
			Area:         area,
			BuildingId:   buildingId,
			CustomType:   customType,
			DeskCode:     deskCode,
			FloorName:    floorName,
			FloorSection: floorSection,
		})
	}

	return locations, nil
}

func attributesToUserPhones(phones map[string]string) ([]admin.UserPhone, error) {
	userPhones := []admin.UserPhone{}

	for key, val := range phones {
		if val == "" {
			continue
		}
		split := strings.Split(key, delim)
		if split[0] != "phone" {
			return userPhones, fmt.Errorf("phone key doesn't start with 'phone': %s", key)
		}
		if len(split) < 2 {
			// for backward compatibility
			userPhones = append(userPhones, admin.UserPhone{Type: "work", Value: val})
			continue
		}
		phoneType := strings.TrimRight(split[1], "~0123456789")
		custom := ""
		if len(split) > 2 && phoneType == "custom" {
			custom = split[2]
		}
		userPhones = append(userPhones, admin.UserPhone{Type: phoneType, CustomType: custom, Value: val})
	}

	return userPhones, nil
}

func updateRelations(newRelation string, oldRelations any) ([]admin.UserRelation, error) {
	relations := []admin.UserRelation{{Type: "manager", Value: newRelation}}

	if oldRelations == nil {
		return relations, nil
	}

	interfaces, ok := oldRelations.([]any)
	if !ok {
		return nil, errors.New("no slice in Google API Relations")
	}

	for i := range interfaces {
		relationMap, ok := interfaces[i].(map[string]any)
		if !ok {
			return nil, errors.New("unexpected data in Google API relation list")
		}

		thisType, ok := relationMap["type"].(string)
		if !ok {
			return nil, errors.New("unexpected data in Google API relation list entry")
		}

		if thisType == "manager" {
			continue
		}

		value, _ := relationMap["value"].(string)
		customType, _ := relationMap["customType"].(string)
		relations = append(relations, admin.UserRelation{
			Type:       thisType,
			Value:      value,
			CustomType: customType,
		})
	}

	return relations, nil
}
