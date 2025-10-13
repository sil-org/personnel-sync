package google

import (
	"reflect"
	"testing"

	"github.com/sil-org/personnel-sync/v6/internal"

	admin "google.golang.org/api/admin/directory/v1"
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
