package google

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	admin "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"

	"github.com/sil-org/personnel-sync/v7/internal"
)

func TestGoogleGroups_ApplyChangeSet(t *testing.T) {
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
			name: "expect two created, one deleted",
			want: internal.ChangeResults{
				Created: uint64(0),
				Deleted: uint64(1),
			},
			fields: fields{
				DestinationConfig: testConfig.Destination,
			},
			args: args{
				changes: internal.ChangeSet{
					Delete: []internal.Person{},
					Create: []internal.Person{},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGoogleGroupsDestination(tt.fields.DestinationConfig)
			if err != nil {
				t.Fatalf("Failed to get new googleGroups instance, error: %s", err.Error())
			}
			eventLog := make(chan internal.EventLogItem, 50)
			if got := g.ApplyChangeSet(tt.args.changes, eventLog); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GoogleGroups.ApplyChangeSet() = %v, want %v", got, tt.want)
			}
			close(eventLog)
		})
	}
}

func TestGoogleGroups_ListUsers(t *testing.T) {
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
		DestinationConfig  internal.DestinationConfig
		GoogleGroupsConfig GoogleConfig
		AdminService       admin.Service
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
			want:    []internal.Person{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g, err := NewGoogleGroupsDestination(tt.fields.DestinationConfig)
			if err != nil {
				t.Fatalf("Failed to get new googleGroups instance, error: %s", err.Error())
			}
			got, err := g.ListUsers([]string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("GoogleGroups.ListUsers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GoogleGroups.ListUsers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGoogleGroups_ExtraMembersNotReAdded(t *testing.T) {
	var mu sync.Mutex
	var insertedEmails []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			resp := admin.Members{
				Members: []*admin.Member{
					{Email: "regular_member@sil.org", Role: RoleMember},
					{Email: "Google_Admin@sil.org", Role: RoleMember},
					{Email: "overlap_extra@sil.org", Role: RoleMember},
				},
			}
			require.NoError(t, json.NewEncoder(w).Encode(resp))
		case http.MethodPost:
			var m admin.Member
			require.NoError(t, json.NewDecoder(r.Body).Decode(&m))
			mu.Lock()
			insertedEmails = append(insertedEmails, m.Email)
			mu.Unlock()
			require.NoError(t, json.NewEncoder(w).Encode(m))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	adminSvc, err := admin.NewService(
		context.Background(),
		option.WithEndpoint(server.URL),
		option.WithHTTPClient(server.Client()),
	)
	require.NoError(t, err)

	g := &GoogleGroups{
		AdminService:      *adminSvc,
		BatchSize:         10,
		BatchDelaySeconds: 1,
	}

	syncSetJSON := json.RawMessage(`{
		"GroupEmail": "test_group@groups.sil.org",
		"ExtraMembers": [
			"google_admin@sil.org",
			"overlap_extra@sil.org",
			"missing_extra@sil.org"
		]
	}`)
	require.NoError(t, g.ForSet(syncSetJSON))

	users, err := g.ListUsers([]string{"Email"})
	require.NoError(t, err)
	require.Equal(t, []internal.Person{
		{
			CompareValue: "regular_member@sil.org",
			Attributes: map[string]string{
				"Email": "regular_member@sil.org",
			},
		},
	}, users)
	require.ElementsMatch(t, []string{"Google_Admin@sil.org", "overlap_extra@sil.org"}, g.foundExtraMembers)

	eventLog := make(chan internal.EventLogItem, 50)
	results := g.ApplyChangeSet(internal.ChangeSet{
		Create: []internal.Person{
			{CompareValue: "new_source_user@sil.org"},
			{CompareValue: "overlap_extra@sil.org"},
		},
	}, eventLog)
	close(eventLog)

	require.Equal(t, uint64(2), results.Created)
	require.ElementsMatch(t, []string{"new_source_user@sil.org", "missing_extra@sil.org"}, insertedEmails)

	// Calling ForSet for a subsequent group should reset foundExtraMembers
	require.NoError(t, g.ForSet(json.RawMessage(`{"GroupEmail": "next_group@groups.sil.org"}`)))
	require.Empty(t, g.foundExtraMembers)
}

