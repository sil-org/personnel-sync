package google

import (
	"encoding/json"
	"encoding/xml"
	"reflect"
	"strings"
	"testing"

	"github.com/sil-org/personnel-sync/v7/internal"
)

func TestNewGoogleContactsDestination(t *testing.T) {
	const extraJSON = `{
      "BatchSize": 5,
      "BatchDelaySeconds": 1,
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
		want              GoogleContacts
		wantErr           bool
	}{
		{
			name: "test 1",
			destinationConfig: internal.DestinationConfig{
				Type:          internal.DestinationTypeGoogleContacts,
				DisableAdd:    true,
				DisableDelete: true,
				DisableUpdate: true,
				ExtraJSON:     json.RawMessage(extraJSON),
			},
			want: GoogleContacts{
				BatchSize:         5,
				BatchDelaySeconds: 1,
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
			got, err := NewGoogleContactsDestination(tt.destinationConfig)
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
			g := got.(*GoogleContacts)
			if !reflect.DeepEqual(g.GoogleConfig, tt.want.GoogleConfig) {
				t.Errorf("incorrect GoogleConfig \ngot: %#v, \nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestGoogleContacts_extractPersonsFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		contacts []Contact
		want     []internal.Person
		wantErr  bool
	}{
		{
			name: "no data",
			want: []internal.Person{},
		},
		{
			name: "one contact, all fields",
			contacts: []Contact{
				{
					XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "entry"},
					Links: []Link{
						{
							XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "link"},
							Rel:     "http://schemas.google.com/contacts/2008/rel#photo",
							Href:    "https://www.google.com/m8/feeds/photos/media/example.org/8a415cec04b8a4b8",
						},
						{
							XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "link"},
							Rel:     "self",
							Href:    "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605",
						},
						{
							XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "link"},
							Rel:     "edit",
							Href:    "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605/1585068827106000",
						},
					},
					Etag:  "686897696a7c876b7e",
					Title: "Alfred E. Newman",
					Name: Name{
						XMLName:    xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "name"},
						FullName:   "Alfred E. Newman",
						GivenName:  "Alfred",
						FamilyName: "Newman",
					},
					Emails: []Email{
						{
							XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "email"},
							Address: "alfred@example.com",
							Primary: true,
						},
					},
					PhoneNumbers: []PhoneNumber{
						{
							XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "phoneNumber"},
							Label:   "Personal Phone",
							Value:   "555-1212",
							Primary: true,
						},
						{
							XMLName: xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "phoneNumber"},
							Rel:     relPhoneWork,
							Value:   "123-4567",
							Primary: false,
						},
					},
					Organization: Organization{
						XMLName:        xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "organization"},
						Name:           "Mad Magazine",
						Title:          "Mascot",
						JobDescription: "Photo ops",
						Department:     "Marketing",
					},
					Where: Where{
						XMLName:     xml.Name{Space: "http://www.w3.org/2005/Atom", Local: "where"},
						ValueString: "some place",
					},
					Notes: "some notes",
				},
			},
			want: []internal.Person{
				{
					CompareValue: "alfred@example.com",
					ID:           "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605",
					Attributes: map[string]string{
						contactFieldEmail:          "alfred@example.com",
						contactFieldPhoneNumber:    "555-1212",
						contactFieldFullName:       "Alfred E. Newman",
						contactFieldGivenName:      "Alfred",
						contactFieldFamilyName:     "Newman",
						contactFieldID:             "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605",
						contactFieldOrganization:   "Mad Magazine",
						contactFieldTitle:          "Mascot",
						contactFieldJobDescription: "Photo ops",
						contactFieldDepartment:     "Marketing",
						contactFieldWhere:          "some place",
						contactFieldNotes:          "some notes",
						contactFieldPhoneNumber + delim + "http://schemas.google.com/g/2005#work": "123-4567",
						contactFieldPhoneNumber + delim + "Personal Phone":                        "555-1212",
					},
					DisableChanges: false,
				},
			},
		},
		{
			name: "multiple contacts",
			contacts: []Contact{
				{
					Links:  []Link{{Rel: "self", Href: "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605"}},
					Emails: []Email{{Address: "alfred@example.com", Primary: true}},
				},
				{
					Links:  []Link{{Rel: "self", Href: "https://www.google.com/m8/feeds/contacts/example.org/full/8f47da821e4824d8"}},
					Emails: []Email{{Address: "ironman@example.com", Primary: true}},
				},
			},
			want: []internal.Person{
				{
					CompareValue: "alfred@example.com",
					ID:           "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605",
					Attributes: map[string]string{
						contactFieldEmail:          "alfred@example.com",
						contactFieldPhoneNumber:    "",
						contactFieldFullName:       "",
						contactFieldGivenName:      "",
						contactFieldFamilyName:     "",
						contactFieldID:             "https://www.google.com/m8/feeds/contacts/example.org/full/204e599dcd6d3605",
						contactFieldOrganization:   "",
						contactFieldTitle:          "",
						contactFieldJobDescription: "",
						contactFieldDepartment:     "",
						contactFieldWhere:          "",
						contactFieldNotes:          "",
					},
					DisableChanges: false,
				},
				{
					CompareValue: "ironman@example.com",
					ID:           "https://www.google.com/m8/feeds/contacts/example.org/full/8f47da821e4824d8",
					Attributes: map[string]string{
						contactFieldEmail:          "ironman@example.com",
						contactFieldPhoneNumber:    "",
						contactFieldFullName:       "",
						contactFieldGivenName:      "",
						contactFieldFamilyName:     "",
						contactFieldID:             "https://www.google.com/m8/feeds/contacts/example.org/full/8f47da821e4824d8",
						contactFieldOrganization:   "",
						contactFieldTitle:          "",
						contactFieldJobDescription: "",
						contactFieldDepartment:     "",
						contactFieldWhere:          "",
						contactFieldNotes:          "",
					},
					DisableChanges: false,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := &GoogleContacts{}
			got, err := g.extractPersonsFromResponse(tt.contacts)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractPersonsFromResponse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractPersonsFromResponse() \ngot: %#v, \nwant: %#v", got, tt.want)
			}
		})
	}
}

func TestGoogleContacts_createBody(t *testing.T) {
	tests := []struct {
		name   string
		person internal.Person
		want   string
	}{
		{
			name:   "fullName",
			person: internal.Person{Attributes: map[string]string{contactFieldFullName: "Fred J. Smith"}},
			want:   "<gd:fullName>Fred J. Smith</gd:fullName>",
		},
		{
			name:   "givenName",
			person: internal.Person{Attributes: map[string]string{contactFieldGivenName: "Fred"}},
			want:   "<gd:givenName>Fred</gd:givenName>",
		},
		{
			name:   "familyName",
			person: internal.Person{Attributes: map[string]string{contactFieldFamilyName: "Smith"}},
			want:   "<gd:familyName>Smith</gd:familyName>",
		},
		{
			name:   "email",
			person: internal.Person{Attributes: map[string]string{contactFieldEmail: "fred@example.com"}},
			want:   `<gd:email rel="` + relPhoneWork + `" primary="true" address="fred@example.com">`,
		},
		{
			name:   "phoneNumber",
			person: internal.Person{Attributes: map[string]string{contactFieldPhoneNumber: "555-1212"}},
			want:   `<gd:phoneNumber rel="` + relPhoneWork + `" primary="true">555-1212</gd:phoneNumber>`,
		},
		{
			name: "phoneNumber+rel",
			person: internal.Person{Attributes: map[string]string{
				contactFieldPhoneNumber + delim + relPhoneMobile: "555-1212",
			}},
			want: `<gd:phoneNumber rel="` + relPhoneMobile + `">555-1212</gd:phoneNumber>`,
		},
		{
			name: "phoneNumber+label",
			person: internal.Person{Attributes: map[string]string{
				contactFieldPhoneNumber + delim + "a label": "555-1212",
			}},
			want: `<gd:phoneNumber label="a label">555-1212</gd:phoneNumber>`,
		},
		{
			name:   "organization",
			person: internal.Person{Attributes: map[string]string{contactFieldOrganization: "Acme, Inc."}},
			want:   "<gd:orgName>Acme, Inc.</gd:orgName>",
		},
		{
			name:   "department",
			person: internal.Person{Attributes: map[string]string{contactFieldDepartment: "Operations"}},
			want:   "<gd:orgDepartment>Operations</gd:orgDepartment>",
		},
		{
			name:   "title",
			person: internal.Person{Attributes: map[string]string{contactFieldTitle: "VP of Operations"}},
			want:   "<gd:orgTitle>VP of Operations</gd:orgTitle>",
		},
		{
			name:   "jobDescription",
			person: internal.Person{Attributes: map[string]string{contactFieldJobDescription: "does important stuff"}},
			want:   "<gd:orgJobDescription>does important stuff</gd:orgJobDescription>",
		},
		{
			name:   "where",
			person: internal.Person{Attributes: map[string]string{contactFieldWhere: "building A"}},
			want:   `<gd:where valueString="building A">`,
		},
		{
			name:   "notes",
			person: internal.Person{Attributes: map[string]string{contactFieldNotes: "these are some notes"}},
			want:   `<atom:content type="text">these are some notes</atom:content>`,
		},
	}
	for _, tt := range tests {
		g := GoogleContacts{}
		t.Run(tt.name, func(t *testing.T) {
			body, err := g.createBody(tt.person)
			if err != nil {
				t.Errorf("unexpected error in createBody: %s", err)
				return
			}
			if !strings.Contains(body, tt.want) {
				t.Errorf("no '%v' in body: \n%v", tt.want, body)
				return
			}
			if !strings.HasPrefix(body, `<atom:entry xmlns:atom="http://www.w3.org/2005/Atom" xmlns:gd="http://schemas.google.com/g/2005">`) {
				t.Errorf("missing <atom:entry> tag in body: \n%s", body)
				return
			}
			if !strings.Contains(body, `<atom:category scheme="http://schemas.google.com/g/2005#kind" term="http://schemas.google.com/contact/2008#contact">`) {
				t.Errorf("missing <atom:category> tag in body: \n%s", body)
				return
			}
		})
	}
}

func TestGoogleContacts_getPhonesFromAttributes(t *testing.T) {
	tests := []struct {
		name       string
		attributes map[string]string
		want       []phoneNumberMarshal
	}{
		{
			name:       "no attributes",
			attributes: nil,
			want:       nil,
		},
		{
			name: "no phone attributes",
			attributes: map[string]string{
				"key": "val",
			},
			want: nil,
		},
		{
			name: "only primary phone",
			attributes: map[string]string{
				contactFieldPhoneNumber: "555-1212",
			},

			want: []phoneNumberMarshal{{
				Rel:     relPhoneWork,
				Primary: true,
				Value:   "555-1212",
			}},
		},
		{
			name: "two phones, one is primary",
			attributes: map[string]string{
				contactFieldPhoneNumber:                          "555-1212",
				contactFieldPhoneNumber + delim + relPhoneMobile: "123-4567",
			},

			want: []phoneNumberMarshal{
				{
					Rel:     relPhoneWork,
					Primary: true,
					Value:   "555-1212",
				},
				{
					Rel:     relPhoneMobile,
					Primary: false,
					Value:   "123-4567",
				},
			},
		},
		{
			name: "two phones, no primary",
			attributes: map[string]string{
				contactFieldPhoneNumber + delim + relPhoneWork:   "555-1212",
				contactFieldPhoneNumber + delim + relPhoneMobile: "123-4567",
			},

			want: []phoneNumberMarshal{
				{
					Rel:     relPhoneWork,
					Primary: false,
					Value:   "555-1212",
				},
				{
					Rel:     relPhoneMobile,
					Primary: false,
					Value:   "123-4567",
				},
			},
		},
		{
			name: "with label",
			attributes: map[string]string{
				contactFieldPhoneNumber + delim + "label text": "555-1212",
			},

			want: []phoneNumberMarshal{
				{
					Primary: false,
					Label:   "label text",
					Value:   "555-1212",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getPhonesFromAttributes(tt.attributes)
			if len(got) != len(tt.want) {
				t.Errorf("wrong number of phoneNumbers returned from getPhonesFromAttributes")
				return
			}
			for _, w := range tt.want {
				found := false
				for _, g := range got {
					if reflect.DeepEqual(g, w) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("getPhonesFromAttributes did not find: %#v", w)
				}
			}
		})
	}
}
