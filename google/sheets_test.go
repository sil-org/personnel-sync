package google

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sil-org/personnel-sync/v7/internal"
)

func TestNewGoogleSheetsDestination(t *testing.T) {
	const extraJSON = `{
      "DelegatedAdminEmail": "delegated-admin@example.com",
      "Domain": "example.com",
      "GoogleAuth": {
        "type": "service_account",
        "project_id": "abc-theme-123456",
        "private_key_id": "abc123",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
        "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
        "client_id": "123456789012345678901",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
      }
	}`

	tests := []struct {
		name              string
		destinationConfig internal.DestinationConfig
		want              GoogleSheets
		wantErr           bool
	}{
		{
			name: "test 1",
			destinationConfig: internal.DestinationConfig{
				Type:          internal.DestinationTypeGoogleSheets,
				DisableAdd:    true,
				DisableDelete: true,
				DisableUpdate: true,
				ExtraJSON:     json.RawMessage(extraJSON),
			},
			want: GoogleSheets{
				GoogleConfig: GoogleConfig{
					DelegatedAdminEmail: "delegated-admin@example.com",
					Domain:              "example.com",
					GoogleAuth: GoogleAuth{
						Type:                    "service_account",
						ProjectID:               "abc-theme-123456",
						PrivateKeyID:            "abc123",
						PrivateKey:              "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
						ClientEmail:             "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
						ClientID:                "123456789012345678901",
						AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
						TokenURI:                "https://oauth2.googleapis.com/token",
						AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
						ClientX509CertURL:       "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "wrong type",
			destinationConfig: internal.DestinationConfig{
				Type:          internal.DestinationTypeGoogleGroups,
				DisableAdd:    true,
				DisableDelete: true,
				DisableUpdate: true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewGoogleSheetsDestination(tt.destinationConfig)
			if tt.wantErr {
				if err == nil {
					t.Errorf("error expected, but didn't happen")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %s", err)
				return
			}
			g := got.(*GoogleSheets)
			if !reflect.DeepEqual(g.GoogleConfig, tt.want.GoogleConfig) {
				t.Errorf("incorrect GoogleConfig \ngot: %#v, \nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestNewGoogleSheetsSource(t *testing.T) {
	const extraJSON = `{
      "DelegatedAdminEmail": "delegated-admin@example.com",
      "Domain": "example.com",
      "GoogleAuth": {
        "type": "service_account",
        "project_id": "abc-theme-123456",
        "private_key_id": "abc123",
        "private_key": "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
        "client_email": "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
        "client_id": "123456789012345678901",
        "auth_uri": "https://accounts.google.com/o/oauth2/auth",
        "token_uri": "https://oauth2.googleapis.com/token",
        "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
        "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com"
      }
	}`

	tests := []struct {
		name         string
		sourceConfig internal.SourceConfig
		want         GoogleSheets
		wantErr      bool
	}{
		{
			name: "test 1",
			sourceConfig: internal.SourceConfig{
				Type:      internal.SourceTypeGoogleSheets,
				ExtraJSON: json.RawMessage(extraJSON),
			},
			want: GoogleSheets{
				GoogleConfig: GoogleConfig{
					DelegatedAdminEmail: "delegated-admin@example.com",
					Domain:              "example.com",
					GoogleAuth: GoogleAuth{
						Type:                    "service_account",
						ProjectID:               "abc-theme-123456",
						PrivateKeyID:            "abc123",
						PrivateKey:              "-----BEGIN PRIVATE KEY-----\nMIIabc...\nabc...\n...xyz\n-----END PRIVATE KEY-----\n",
						ClientEmail:             "my-sync-bot@abc-theme-123456.iam.gserviceaccount.com",
						ClientID:                "123456789012345678901",
						AuthURI:                 "https://accounts.google.com/o/oauth2/auth",
						TokenURI:                "https://oauth2.googleapis.com/token",
						AuthProviderX509CertURL: "https://www.googleapis.com/oauth2/v1/certs",
						ClientX509CertURL:       "https://www.googleapis.com/robot/v1/metadata/x509/my-sync-bot%40abc-theme-123456.iam.gserviceaccount.com",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "wrong type",
			sourceConfig: internal.SourceConfig{
				Type: internal.SourceTypeRestAPI,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewGoogleSheetsSource(tt.sourceConfig)
			if tt.wantErr {
				if err == nil {
					t.Errorf("error expected, but didn't happen")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %s", err)
				return
			}
			g := got.(*GoogleSheets)
			if !reflect.DeepEqual(g.GoogleConfig, tt.want.GoogleConfig) {
				t.Errorf("incorrect GoogleConfig \ngot: %#v, \nwant: %#v", got, tt.want)
			}
		})
	}
}

func Test_getPersonsFromSheetData(t *testing.T) {
	tests := []struct {
		name         string
		sheetData    [][]any
		desiredAttrs []string
		compareAttr  string
		want         []internal.Person
	}{
		{
			name:         "empty sheet data",
			sheetData:    [][]any{},
			desiredAttrs: []string{"a", "b"},
			want:         []internal.Person{},
		},
		{
			name: "only a header row",
			sheetData: [][]any{
				{"a", "b", "c"},
			},
			desiredAttrs: []string{"a", "b"},
			want:         []internal.Person{},
		},
		{
			name: "no desired attributes",
			sheetData: [][]any{
				{"a", "b", "c"},
				{"valueA", "valueB", "valueC"},
			},
			desiredAttrs: []string{},
			want: []internal.Person{{
				Attributes: map[string]string{},
			}},
		},
		{
			name: "one row",
			sheetData: [][]any{
				{"a", "b", "c"},
				{"valueA", "valueB", "valueC"},
			},
			desiredAttrs: []string{"a", "b"},
			compareAttr:  "b",
			want: []internal.Person{{
				Attributes: map[string]string{
					"a": "valueA",
					"b": "valueB",
				},
				CompareValue: "valueB",
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPersonsFromSheetData(tt.sheetData, tt.desiredAttrs, tt.compareAttr)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getPersonsFromSheetData() = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestGoogleSheets_getHeaderFromSheetData(t *testing.T) {
	tests := []struct {
		name string
		data [][]any
		want map[int]string
	}{
		{
			name: "empty sheet data",
			data: [][]any{},
			want: map[int]string{},
		},
		{
			name: "one column",
			data: [][]any{
				{"a"},
			},
			want: map[int]string{
				0: "a",
			},
		},
		{
			name: "two columns, separated by a an empty column",
			data: [][]any{
				{"a", "", "c"},
			},
			want: map[int]string{
				0: "a",
				1: "",
				2: "c",
			},
		},
		{
			name: "two columns with same value",
			data: [][]any{
				{"a", "a"},
			},
			want: map[int]string{
				0: "a",
				1: "a",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getHeaderFromSheetData(tt.data); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getHeaderFromSheetData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_makeSheetDataFromPersons(t *testing.T) {
	tests := []struct {
		name    string
		header  map[int]string
		persons []internal.Person
		want    [][]any
	}{
		{
			name:    "empty input",
			header:  map[int]string{},
			persons: []internal.Person{},
			want:    [][]any{},
		},
		{
			name:   "empty header",
			header: map[int]string{},
			persons: []internal.Person{
				{
					Attributes:     map[string]string{"a": "valueA"},
					DisableChanges: false,
				},
			},
			want: [][]any{},
		},
		{
			name:    "empty persons list",
			header:  map[int]string{0: "a"},
			persons: []internal.Person{},
			want:    [][]any{},
		},
		{
			name:   "2 persons, 2 attributes",
			header: map[int]string{0: "a", 1: "b"},
			persons: []internal.Person{
				{
					Attributes:     map[string]string{"a": "valueA1", "b": "valueB1"},
					DisableChanges: false,
				},
				{
					Attributes:     map[string]string{"a": "valueA2", "b": "valueB2"},
					DisableChanges: false,
				},
			},
			want: [][]any{
				{"valueA1", "valueB1"},
				{"valueA2", "valueB2"},
			},
		},
		{
			name:   "extra header column",
			header: map[int]string{0: "a", 1: "b", 2: "c"},
			persons: []internal.Person{
				{
					Attributes:     map[string]string{"a": "valueA1", "b": "valueB1"},
					DisableChanges: false,
				},
				{
					Attributes:     map[string]string{"a": "valueA2", "b": "valueB2"},
					DisableChanges: false,
				},
			},
			want: [][]any{
				{"valueA1", "valueB1", ""},
				{"valueA2", "valueB2", ""},
			},
		},
		{
			name:   "unused attribute",
			header: map[int]string{0: "a", 1: "b"},
			persons: []internal.Person{
				{
					Attributes:     map[string]string{"a": "valueA1", "b": "valueB1", "c": "valueC1"},
					DisableChanges: false,
				},
			},
			want: [][]any{
				{"valueA1", "valueB1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := makeSheetDataFromPersons(tt.header, tt.persons); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("makeSheetDataFromPersons() = %v, want %v", got, tt.want)
			}
		})
	}
}
