package webhelpdesk

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/sil-org/personnel-sync/v7/internal"
	"github.com/sil-org/personnel-sync/v7/restapi"
)

func TestWebHelpDesk_ListUsers(t *testing.T) {
	fixtures := map[string][]User{
		"page1": {
			{
				ID:        1,
				FirstName: "c1",
				LastName:  "c1",
				Email:     "c1@c1.com",
				Username:  "c1",
			},
			{
				ID:        2,
				FirstName: "c2",
				LastName:  "c2",
				Email:     "c2@c2.com",
				Username:  "c2",
			},
			{
				ID:        3,
				FirstName: "c3",
				LastName:  "c3",
				Email:     "c3@c3.com",
				Username:  "c3",
			},
			{
				ID:        4,
				FirstName: "c4",
				LastName:  "c4",
				Email:     "c4@c4.com",
				Username:  "c4",
			},
			{
				ID:        5,
				FirstName: "c5",
				LastName:  "c5",
				Email:     "c5@c5.com",
				Username:  "c5",
			},
		},
		"page2": {
			{
				ID:        6,
				FirstName: "c6",
				LastName:  "c6",
				Email:     "c6@c6.com",
				Username:  "c6",
			},
			{
				ID:        7,
				FirstName: "c7",
				LastName:  "c7",
				Email:     "c7@c7.com",
				Username:  "c7",
			},
			{
				ID:        8,
				FirstName: "c8",
				LastName:  "c8",
				Email:     "c8@c8.com",
				Username:  "c8",
			},
			{
				ID:        9,
				FirstName: "c9",
				LastName:  "c9",
				Email:     "c9@c9.com",
				Username:  "c9",
			},
		},
	}

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)

	mux.HandleFunc("/ra/Clients", func(w http.ResponseWriter, req *http.Request) {
		v := req.URL.Query()
		page := v.Get("page")

		key := fmt.Sprintf("page%s", page)

		jsonBytes, err := json.Marshal(fixtures[key])
		if err != nil {
			t.Fatalf("Unable to marshal fixture results, error: %s", err.Error())
		}

		w.WriteHeader(200)
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprint(w, string(jsonBytes))
	})

	whdConfig := WebHelpDesk{
		URL:                  server.URL,
		Password:             "alala",
		Username:             "bkbkb",
		ListClientsPageLimit: 5,
	}

	extraJson, err := json.Marshal(whdConfig)
	if err != nil {
		t.Errorf("Error marshalling whdConfig to json: %s", err.Error())
	}

	type fields struct {
		DestinationConfig internal.DestinationConfig
	}
	tests := []struct {
		name    string
		fields  fields
		want    []internal.Person
		wantErr bool
	}{
		{
			name: "all results",
			fields: fields{
				DestinationConfig: internal.DestinationConfig{
					Type:      internal.DestinationTypeWebHelpDesk,
					ExtraJSON: extraJson,
				},
			},
			want: []internal.Person{
				{
					CompareValue: "c1",
					Attributes: map[string]string{
						"id":        "1",
						"email":     "c1@c1.com",
						"firstName": "c1",
						"lastName":  "c1",
						"username":  "c1",
					},
				},
				{
					CompareValue: "c2",
					Attributes: map[string]string{
						"id":        "2",
						"email":     "c2@c2.com",
						"firstName": "c2",
						"lastName":  "c2",
						"username":  "c2",
					},
				},
				{
					CompareValue: "c3",
					Attributes: map[string]string{
						"id":        "3",
						"email":     "c3@c3.com",
						"firstName": "c3",
						"lastName":  "c3",
						"username":  "c3",
					},
				},
				{
					CompareValue: "c4",
					Attributes: map[string]string{
						"id":        "4",
						"email":     "c4@c4.com",
						"firstName": "c4",
						"lastName":  "c4",
						"username":  "c4",
					},
				},
				{
					CompareValue: "c5",
					Attributes: map[string]string{
						"id":        "5",
						"email":     "c5@c5.com",
						"firstName": "c5",
						"lastName":  "c5",
						"username":  "c5",
					},
				},
				{
					CompareValue: "c6",
					Attributes: map[string]string{
						"id":        "6",
						"email":     "c6@c6.com",
						"firstName": "c6",
						"lastName":  "c6",
						"username":  "c6",
					},
				},
				{
					CompareValue: "c7",
					Attributes: map[string]string{
						"id":        "7",
						"email":     "c7@c7.com",
						"firstName": "c7",
						"lastName":  "c7",
						"username":  "c7",
					},
				},
				{
					CompareValue: "c8",
					Attributes: map[string]string{
						"id":        "8",
						"email":     "c8@c8.com",
						"firstName": "c8",
						"lastName":  "c8",
						"username":  "c8",
					},
				},
				{
					CompareValue: "c9",
					Attributes: map[string]string{
						"id":        "9",
						"email":     "c9@c9.com",
						"firstName": "c9",
						"lastName":  "c9",
						"username":  "c9",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, err := NewWebHelpDeskDestination(tt.fields.DestinationConfig)
			if err != nil {
				t.Fatal(err)
			}
			got, err := w.ListUsers([]string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("WebHelpDesk.ListUsers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("WebHelpDesk.ListUsers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateChangeSet(t *testing.T) {
	t.Skip("Requires integration with WHD so skipped by default")
	t.SkipNow()

	configJSON, err := internal.LoadConfig("./config.json")
	if err != nil {
		t.Fatalf("Failed to load test config, error: %s", err.Error())
	}

	testConfig, err := internal.ReadConfig(configJSON)
	if err != nil {
		t.Fatalf("Failed to read test config, error: %s", err.Error())
	}

	whd, err := NewWebHelpDeskDestination(testConfig.Destination)
	if err != nil {
		t.Fatalf("Failed to get new whd client, error: %s", err.Error())
	}

	users, err := whd.ListUsers([]string{})
	if err != nil {
		t.Fatalf("Failed to list whd users, error: %s", err.Error())
	}

	source, err := restapi.NewRestAPISource(testConfig.Source)
	if err != nil {
		t.Error(err)
	}

	sourcePeople, _ := source.ListUsers([]string{"email"})
	log.Printf("found %v people in source", len(sourcePeople))

	logger := log.New(os.Stdout, "", 0)
	changeSet := internal.GenerateChangeSet(logger, sourcePeople, users, testConfig)

	log.Printf("ChangeSet ready %v to be created, %v to be deleted", len(changeSet.Create), len(changeSet.Delete))
}

func TestWebHelpDesk_CreateUser(t *testing.T) {
	t.Skip("Requires integration with WHD so skipped by default")
	t.SkipNow()

	configJSON, err := internal.LoadConfig("./config.json")
	if err != nil {
		t.Fatalf("Failed to load test config, error: %s", err.Error())
	}

	testConfig, err := internal.ReadConfig(configJSON)
	if err != nil {
		t.Fatalf("Failed to read test config, error: %s", err.Error())
	}

	whd, err := NewWebHelpDeskDestination(testConfig.Destination)
	if err != nil {
		t.Fatalf("Failed to get new whd client, error: %s", err.Error())
	}

	personToCreate := internal.Person{
		Attributes: map[string]string{
			"firstName": "testing123456",
			"lastName":  "test-for-phillip",
			"email":     "phillip_testing123456@domain.com",
			"username":  "phillip_testing123456@domain.com",
		},
	}

	changeSet := internal.ChangeSet{
		Create: []internal.Person{
			personToCreate,
		},
	}

	eventLog := make(chan internal.EventLogItem, 50)
	changeResults := whd.ApplyChangeSet(changeSet, eventLog)
	close(eventLog)
	log.Println(changeResults)

	if changeResults.Created != 1 {
		t.Errorf("Unable to create user, number of users created was %v", changeResults.Created)
		log.Println("Errors creating user:")
	}
}
