package google

import (
	"math/rand"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/api/googleapi"

	"github.com/sil-org/personnel-sync/v7/internal"

	admin "google.golang.org/api/admin/directory/v1"
)

func TestGoogleUsers_ListUsers(t *testing.T) {
	t.Skip("Skipping test because it requires integration with Google")
	t.SkipNow()

	rawConfig, err := internal.LoadConfig("../cmd/config.json")
	if err != nil {
		t.Fatalf("Failed to load test config, error: %s", err.Error())
	}

	testConfig, err := internal.ReadConfig(rawConfig)
	if err != nil {
		t.Fatalf("Failed to read test config, error: %s", err.Error())
	}

	type fields struct {
		DestinationConfig internal.DestinationConfig
		GoogleConfig      GoogleConfig
		AdminService      admin.Service
	}
	tests := []struct {
		name    string
		fields  fields
		want    []internal.Person
		wantErr bool
	}{
		{
			name: "test listing users",
			fields: fields{
				DestinationConfig: testConfig.Destination,
			},
			want: []internal.Person{
				{
					CompareValue: "user_one@example.com",
					Attributes: map[string]string{
						"email":      "user_one@example.com",
						"familyName": "one",
						"fullName":   "user one",
						"givenName":  "user",
					},
					DisableChanges: false,
				},
				{
					CompareValue: "user_two@example.com",
					Attributes: map[string]string{
						"email":      "user_two@example.com",
						"familyName": "two",
						"fullName":   "user two",
						"givenName":  "user",
					},
					DisableChanges: false,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGoogleUsersDestination(tt.fields.DestinationConfig)
			require.NoErrorf(t, err, "Failed to get new googleUsers instance, error: %s", err.Error())
			got, err := g.ListUsers([]string{})
			require.Equalf(t, tt.wantErr, err != nil, "GoogleUsers.ListUsers() error = %v, wantErr %v", err, tt.wantErr)
			require.Equalf(t, got, tt.want, "GoogleUsers.ListUsers() = %v, want %v", got, tt.want)
		})
	}
}

func TestGoogleUsers_ApplyChangeSet(t *testing.T) {
	t.Skip("Skipping test because it requires integration with Google")
	t.SkipNow()

	rawConfig, err := internal.LoadConfig("./config.json")
	if err != nil {
		t.Fatalf("Failed to load test config, error: %s", err.Error())
	}

	testConfig, err := internal.ReadConfig(rawConfig)
	if err != nil {
		t.Fatalf("Failed to read test config, error: %s", err.Error())
	}

	type fields struct {
		DestinationConfig internal.DestinationConfig
	}
	type args struct {
		changes internal.ChangeSet
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   internal.ChangeResults
	}{
		{
			name: "expect one updated",
			want: internal.ChangeResults{
				Created: uint64(0),
				Updated: uint64(1),
				Deleted: uint64(0),
			},
			fields: fields{
				DestinationConfig: testConfig.Destination,
			},
			args: args{
				changes: internal.ChangeSet{
					Create: []internal.Person{},
					Update: []internal.Person{
						{
							CompareValue: "user@example.com",
							Attributes: map[string]string{
								"email":      "user@example.com",
								"familyName": strconv.Itoa(rand.Intn(1000)),
								"givenName":  "x",
							},
							DisableChanges: false,
						},
					},
					Delete: []internal.Person{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGoogleUsersDestination(tt.fields.DestinationConfig)
			require.NoErrorf(t, err, "Failed to get new googleUsers instance, error: %s", err.Error())
			eventLog := make(chan internal.EventLogItem, 50)
			got := g.ApplyChangeSet(tt.args.changes, eventLog)
			require.Equalf(t, tt.want, got, "GoogleUsers.ApplyChangeSet() = %v, want %v", got, tt.want)
			close(eventLog)
		})
	}
}

func TestGoogleUsers_extractData(t *testing.T) {
	tests := []struct {
		name string
		user admin.User
		want internal.Person
	}{
		{
			name: "minimum",
			user: admin.User{
				ExternalIds:   nil,
				Locations:     nil,
				Name:          nil,
				Organizations: nil,
				Phones:        nil,
				PrimaryEmail:  "email@example.com",
				Relations:     nil,
			},
			want: internal.Person{
				CompareValue: "email@example.com",
				Attributes:   map[string]string{"email": "email@example.com"},
			},
		},
		{
			name: "all supported fields",
			user: admin.User{
				ExternalIds: []any{map[string]any{
					"type":  "organization",
					"value": "12345",
				}},
				Locations: []any{map[string]any{
					"area": "An area",
					"type": "desk",
				}},
				Name: &admin.UserName{
					FamilyName: "Jones",
					FullName:   "John Jones",
					GivenName:  "John",
				},
				Organizations: []any{map[string]any{
					"costCenter": "A cost center",
					"department": "A department",
					"title":      "A title",
				}},
				Phones: []any{map[string]any{
					"type":  "work",
					"value": "555-1212",
				}},
				PrimaryEmail: "email@example.com",
				Relations: []any{map[string]any{
					"type":  "manager",
					"value": "manager@example.com",
				}},
				CustomSchemas: map[string]googleapi.RawMessage{
					"Location": []byte(`{"Building":"A building"}`),
				},
			},
			want: internal.Person{
				CompareValue: "email@example.com",
				Attributes: map[string]string{
					"email":                  "email@example.com",
					"familyName":             "Jones",
					"givenName":              "John",
					"id":                     "12345",
					"area":                   "An area",
					"costCenter":             "A cost center",
					"department":             "A department",
					"title":                  "A title",
					"phone" + delim + "work": "555-1212",
					"manager":                "manager@example.com",
					"Location.Building":      "A building",
				},
			},
		},
		{
			name: `only "organization" externalIDs`,
			user: admin.User{
				ExternalIds: []any{
					map[string]any{
						"type":  "custom",
						"value": "abc123",
					},
					map[string]any{
						"type":  "organization",
						"value": "12345",
					},
				},
				PrimaryEmail: "email@example.com",
			},
			want: internal.Person{
				CompareValue: "email@example.com",
				Attributes: map[string]string{
					"email": "email@example.com",
					"id":    "12345",
				},
			},
		},
		{
			name: `only "work" phones`,
			user: admin.User{
				PrimaryEmail: "email@example.com",
				Phones: []any{
					map[string]any{
						"type":  "home",
						"value": "555-1212",
					},
					map[string]any{
						"type":  "work",
						"value": "888-5555",
					},
				},
			},
			want: internal.Person{
				CompareValue: "email@example.com",
				Attributes: map[string]string{
					"email":                  "email@example.com",
					"phone" + delim + "home": "555-1212",
					"phone" + delim + "work": "888-5555",
				},
			},
		},
		{
			name: `only "desk" locations`,
			user: admin.User{
				PrimaryEmail: "email@example.com",
				Locations: []any{
					map[string]any{
						"area": "Custom area",
						"type": "custom",
					},
					map[string]any{
						"area": "An area",
						"type": "desk",
					},
				},
			},
			want: internal.Person{
				CompareValue: "email@example.com",
				Attributes: map[string]string{
					"email": "email@example.com",
					"area":  "An area",
				},
			},
		},
		{
			name: "invalid data types",
			user: admin.User{
				ExternalIds: []any{map[string]any{
					"type":  "organization",
					"value": 12345,
				}},
				Locations: []any{map[string]any{
					"type": "desk",
					"area": 1.0,
				}},
				Organizations: []any{map[string]any{
					"costCenter": []string{"A cost center"},
					"department": true,
					"title":      map[string]string{"key": "value"},
				}},
				Phones: []any{map[string]any{
					"type":  "work",
					"value": 5551212,
				}},
				PrimaryEmail: "email@example.com",
				Relations: []any{map[string]any{
					"type":  "manager",
					"value": []string{"manager@example.com"},
				}},
			},
			want: internal.Person{
				CompareValue: "email@example.com",
				Attributes: map[string]string{
					"email": "email@example.com",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractData(tt.user)
			require.Equalf(t, tt.want, got, "extractData() = %#v\nwant: %#v", got, tt.want)
		})
	}
}

func Test_newUserForUpdate(t *testing.T) {
	tests := []struct {
		name   string
		person internal.Person
		want   admin.User
	}{
		{
			name: "basic",
			person: internal.Person{
				CompareValue: "email@example.com",
				Attributes: map[string]string{
					"email":                  "email@example.com",
					"familyName":             "Jones",
					"givenName":              "John",
					"id":                     "12345",
					"area":                   "An area",
					"costCenter":             "A cost center",
					"department":             "A department",
					"title":                  "A title",
					"phone" + delim + "work": "555-1212",
					"manager":                "manager@example.com",
					"Location.Building":      "A building",
				},
			},
			want: admin.User{
				ExternalIds: []admin.UserExternalId{{
					Type:  "organization",
					Value: "12345",
				}},
				Locations: []admin.UserLocation{{
					Area: "An area",
					Type: "desk",
				}},
				Name: &admin.UserName{
					FamilyName: "Jones",
					GivenName:  "John",
				},
				Organizations: []admin.UserOrganization{{
					CostCenter: "A cost center",
					Department: "A department",
					Title:      "A title",
				}},
				Phones: []admin.UserPhone{{
					Type:  "work",
					Value: "555-1212",
				}},
				Relations: []admin.UserRelation{{
					Type:  "manager",
					Value: "manager@example.com",
				}},
				CustomSchemas: map[string]googleapi.RawMessage{
					"Location": []byte(`{"Building":"A building"}`),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := newUserForUpdate(tt.person, admin.User{})
			require.NoError(t, err)
			require.Equalf(t, tt.want, got, "newUserForUpdate() = %#v\nwant: %#v", got, tt.want)
		})
	}
}

func Test_updateIDs(t *testing.T) {
	tests := []struct {
		name   string
		newID  string
		oldIDs any
		want   []admin.UserExternalId
	}{
		{
			name:  "organization and custom",
			newID: "12345",
			oldIDs: []any{
				map[string]any{
					"type":  "organization",
					"value": "00000",
				},
				map[string]any{
					"type":       "custom",
					"customType": "foo",
					"value":      "abcdef",
				},
			},
			want: []admin.UserExternalId{
				{
					Type:  "organization",
					Value: "12345",
				},
				{
					Type:       "custom",
					Value:      "abcdef",
					CustomType: "foo",
				},
			},
		},
		{
			name:  "organization only",
			newID: "12345",
			oldIDs: []any{
				map[string]any{
					"type":  "organization",
					"value": "00000",
				},
			},
			want: []admin.UserExternalId{
				{
					Type:  "organization",
					Value: "12345",
				},
			},
		},
		{
			name:  "custom only",
			newID: "12345",
			oldIDs: []any{
				map[string]any{
					"type":       "custom",
					"customType": "foo",
					"value":      "abcdef",
				},
			},
			want: []admin.UserExternalId{
				{
					Type:  "organization",
					Value: "12345",
				},
				{
					Type:       "custom",
					Value:      "abcdef",
					CustomType: "foo",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := updateIDs(tt.newID, tt.oldIDs)
			require.NoError(t, err, "updateIDs() error")
			require.Equal(t, tt.want, got, "updateIDs():\n%+v\nwant:\n%+v", got, tt.want)
		})
	}
}

func Test_updateLocations(t *testing.T) {
	tests := []struct {
		name         string
		newArea      string
		oldLocations any
		want         []admin.UserLocation
	}{
		{
			name:    "desk and custom",
			newArea: "Area 2",
			oldLocations: []any{
				map[string]any{
					"type": "desk",
					"area": "Area 1",
				},
				map[string]any{
					"type":         "custom",
					"customType":   "foo",
					"area":         "Area A",
					"buildingId":   "Bldg B",
					"deskCode":     "deskCode",
					"floorName":    "floorName",
					"floorSection": "floorSection",
				},
			},
			want: []admin.UserLocation{
				{
					Type: "desk",
					Area: "Area 2",
				},
				{
					Type:         "custom",
					CustomType:   "foo",
					Area:         "Area A",
					BuildingId:   "Bldg B",
					DeskCode:     "deskCode",
					FloorName:    "floorName",
					FloorSection: "floorSection",
				},
			},
		},
		{
			name:    "desk only",
			newArea: "Area 2",
			oldLocations: []any{
				map[string]any{
					"type": "desk",
					"area": "Area 1",
				},
			},
			want: []admin.UserLocation{
				{
					Type: "desk",
					Area: "Area 2",
				},
			},
		},
		{
			name:    "custom only",
			newArea: "Area 2",
			oldLocations: []any{
				map[string]any{
					"type":         "custom",
					"customType":   "foo",
					"area":         "Area A",
					"buildingId":   "Bldg B",
					"deskCode":     "deskCode",
					"floorName":    "floorName",
					"floorSection": "floorSection",
				},
			},
			want: []admin.UserLocation{
				{
					Type: "desk",
					Area: "Area 2",
				},
				{
					Type:         "custom",
					CustomType:   "foo",
					Area:         "Area A",
					BuildingId:   "Bldg B",
					DeskCode:     "deskCode",
					FloorName:    "floorName",
					FloorSection: "floorSection",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := updateLocations(tt.newArea, tt.oldLocations)
			require.NoError(t, err)
			require.Equal(t, tt.want, got, "updateLocations():\n%+v\nwant:\n%+v", got, tt.want)
		})
	}
}

func Test_updatePhones(t *testing.T) {
	tests := []struct {
		name    string
		phones  map[string]string
		want    []admin.UserPhone
		wantErr bool
	}{
		{
			name: "work+custom",
			phones: map[string]string{
				"phone" + delim + "work":                   "1",
				"phone" + delim + "custom" + delim + "foo": "2",
			},
			want: []admin.UserPhone{
				{
					Type:  "work",
					Value: "1",
				},
				{
					Type:       "custom",
					CustomType: "foo",
					Value:      "2",
				},
			},
		},
		{
			name: "work*3",
			phones: map[string]string{
				"phone" + delim + "work":   "1",
				"phone" + delim + "work~1": "2",
				"phone" + delim + "work~2": "3",
			},
			want: []admin.UserPhone{
				{
					Type:  "work",
					Value: "1",
				},
				{
					Type:  "work",
					Value: "2",
				},
				{
					Type:  "work",
					Value: "3",
				},
			},
		},
		{
			name: "custom*3",
			phones: map[string]string{
				"phone" + delim + "custom" + delim + "other":   "1",
				"phone" + delim + "custom~1" + delim + "other": "2",
				"phone" + delim + "custom~2" + delim + "other": "3",
			},
			want: []admin.UserPhone{
				{
					Type:       "custom",
					CustomType: "other",
					Value:      "1",
				},
				{
					Type:       "custom",
					CustomType: "other",
					Value:      "2",
				},
				{
					Type:       "custom",
					CustomType: "other",
					Value:      "3",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := attributesToUserPhones(tt.phones)
			require.Equal(t, tt.wantErr, err != nil,
				"attributesToUserPhones() error = %v, wantErr %v", err, tt.wantErr)

			require.Equal(t, len(tt.want), len(got),
				"got wrong number of phones (got %d, want %d)", len(got), len(tt.want))

			for w := range tt.want {
				found := false
				for g := range got {
					if reflect.DeepEqual(w, g) {
						found = true
						continue
					}
				}
				require.True(t, found, "didn't find %v in phone list", w)
			}
		})
	}
}

func Test_updateRelations(t *testing.T) {
	tests := []struct {
		name         string
		newRelation  string
		oldRelations any
		want         []admin.UserRelation
	}{
		{
			name:        "manager and custom",
			newRelation: "new_manager@example.com",
			oldRelations: []any{
				map[string]any{
					"type":  "manager",
					"value": "old_manager@example.com",
				},
				map[string]any{
					"type":       "custom",
					"customType": "foo",
					"value":      "other@example.com",
				},
			},
			want: []admin.UserRelation{
				{
					Type:  "manager",
					Value: "new_manager@example.com",
				},
				{
					Type:       "custom",
					CustomType: "foo",
					Value:      "other@example.com",
				},
			},
		},
		{
			name:        "manager only",
			newRelation: "new_manager@example.com",
			oldRelations: []any{
				map[string]any{
					"type":  "manager",
					"value": "old_manager@example.com",
				},
			},
			want: []admin.UserRelation{
				{
					Type:  "manager",
					Value: "new_manager@example.com",
				},
			},
		},
		{
			name:        "custom only",
			newRelation: "new_manager@example.com",
			oldRelations: []any{
				map[string]any{
					"type":       "custom",
					"customType": "foo",
					"value":      "other@example.com",
				},
			},
			want: []admin.UserRelation{
				{
					Type:  "manager",
					Value: "new_manager@example.com",
				},
				{
					Type:       "custom",
					CustomType: "foo",
					Value:      "other@example.com",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := updateRelations(tt.newRelation, tt.oldRelations)
			require.NoError(t, err, "updateRelations() error")
			require.Equal(t, tt.want, got, "updateRelations():\n%+v\nwant:\n%+v", got, tt.want)
		})
	}
}

func Test_getPhoneNumbersFromUser(t *testing.T) {
	tests := []struct {
		name string
		user admin.User
		want map[string]string
	}{
		{
			name: "work+custom",
			user: admin.User{
				Phones: []any{
					map[string]any{
						"type":  "work",
						"value": "555-1212",
					},
					map[string]any{
						"type":       "custom",
						"customType": "foo",
						"value":      "2",
					},
				},
			},
			want: map[string]string{
				"phone" + delim + "work":                   "555-1212",
				"phone" + delim + "custom" + delim + "foo": "2",
			},
		},
		{
			name: "work*3",
			user: admin.User{
				Phones: []any{
					map[string]any{
						"type":  "work",
						"value": "555-1212",
					},
					map[string]any{
						"type":  "work",
						"value": "123-4567",
					},
					map[string]any{
						"type":  "work",
						"value": "999-999-9999",
					},
				},
			},
			want: map[string]string{
				"phone" + delim + "work":   "555-1212",
				"phone" + delim + "work~1": "123-4567",
				"phone" + delim + "work~2": "999-999-9999",
			},
		},
		{
			name: "custom*3",
			user: admin.User{
				Phones: []any{
					map[string]any{
						"type":       "custom",
						"customType": "other",
						"value":      "555-1212",
					},
					map[string]any{
						"type":       "custom",
						"customType": "other",
						"value":      "123-4567",
					},
					map[string]any{
						"type":       "custom",
						"customType": "other",
						"value":      "999-999-9999",
					},
				},
			},
			want: map[string]string{
				"phone" + delim + "custom" + delim + "other":   "555-1212",
				"phone" + delim + "custom~1" + delim + "other": "123-4567",
				"phone" + delim + "custom~2" + delim + "other": "999-999-9999",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPhoneNumbersFromUser(tt.user)
			require.Equal(t, tt.want, got, "getPhoneNumbersFromUser():\n  %v\nwant:\n  %v\n", got, tt.want)
		})
	}
}
