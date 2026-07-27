package google

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"log/syslog"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"golang.org/x/net/context"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"

	"github.com/sil-org/personnel-sync/v7/internal"
)

const MaxQuerySize = 10000

const (
	contactFieldID             = "id"
	contactFieldEmail          = "email"
	contactFieldPhoneNumber    = "phoneNumber"
	contactFieldFullName       = "fullName"
	contactFieldGivenName      = "givenName"
	contactFieldFamilyName     = "familyName"
	contactFieldWhere          = "where"
	contactFieldOrganization   = "organization"
	contactFieldTitle          = "title"
	contactFieldJobDescription = "jobDescription"
	contactFieldDepartment     = "department"
	contactFieldNotes          = "notes"
)

const delim = ","

const (
	relPhoneWork   = "http://schemas.google.com/g/2005#work"
	relPhoneMobile = "http://schemas.google.com/g/2005#mobile"
)

type GoogleContacts struct {
	BatchSize         int
	BatchDelaySeconds int
	DestinationConfig internal.DestinationConfig
	GoogleConfig      GoogleConfig
	Client            http.Client
}

type Entries struct {
	XMLName xml.Name  `xml:"feed"`
	Entries []Contact `xml:"entry"`
	Total   int       `xml:"totalResults"`
}

type Contact struct {
	XMLName      xml.Name      `xml:"entry"`
	ID           string        `xml:"id"`
	Links        []Link        `xml:"link"`
	Etag         string        `xml:"etag,attr"`
	Title        string        `xml:"title"`
	Name         Name          `xml:"name"`
	Emails       []Email       `xml:"email"`
	PhoneNumbers []PhoneNumber `xml:"phoneNumber"`
	Organization Organization  `xml:"organization"`
	Where        Where         `xml:"where"`
	Notes        string        `xml:"content"`
}

type Email struct {
	XMLName xml.Name `xml:"email"`
	Primary bool     `xml:"primary,attr"`
	Address string   `xml:"address,attr"`
}

type PhoneNumber struct {
	XMLName xml.Name `xml:"phoneNumber"`
	Rel     string   `xml:"rel,attr"`
	Label   string   `xml:"label,attr"`
	Primary bool     `xml:"primary,attr"`
	Value   string   `xml:",chardata"`
}

type Name struct {
	XMLName    xml.Name `xml:"name"`
	FullName   string   `xml:"fullName"`
	GivenName  string   `xml:"givenName"`
	FamilyName string   `xml:"familyName"`
}

type Organization struct {
	XMLName        xml.Name `xml:"organization"`
	Name           string   `xml:"orgName"`
	Title          string   `xml:"orgTitle"`
	JobDescription string   `xml:"orgJobDescription"`
	Department     string   `xml:"orgDepartment"`
}

type Link struct {
	XMLName xml.Name `xml:"link"`
	Rel     string   `xml:"rel,attr"`
	Href    string   `xml:"href,attr"`
}

type Where struct {
	XMLName     xml.Name `xml:"where"`
	ValueString string   `xml:"valueString,attr"`
}

// These "Marshal" types included because the Go XML encoder isn't able to use the same struct for marshalling and
// unmarshalling with namespace prefixes
type contactMarshal struct {
	XMLName      xml.Name             `xml:"atom:entry"`
	XmlNSAtom    string               `xml:"xmlns:atom,attr"`
	XmlNSGd      string               `xml:"xmlns:gd,attr"`
	AtomCategory atomCategory         `xml:"atom:category"`
	Name         nameMarshal          `xml:"gd:name"`
	Emails       []emailMarshal       `xml:"gd:email"`
	PhoneNumbers []phoneNumberMarshal `xml:"gd:phoneNumber"`
	Organization organizationMarshal  `xml:"gd:organization"`
	Where        whereMarshal         `xml:"gd:where"`
	Notes        notesMarshal         `xml:"atom:content"`
}

type atomCategory struct {
	Scheme string `xml:"scheme,attr"`
	Term   string `xml:"term,attr"`
}

type emailMarshal struct {
	Rel     string `xml:"rel,attr"`
	Primary bool   `xml:"primary,attr,omitempty"`
	Address string `xml:"address,attr"`
}

type phoneNumberMarshal struct {
	Rel     string `xml:"rel,attr,omitempty"`
	Label   string `xml:"label,attr,omitempty"`
	Primary bool   `xml:"primary,attr,omitempty"`
	Value   string `xml:",chardata"`
}

type nameMarshal struct {
	FullName   string `xml:"gd:fullName"`
	GivenName  string `xml:"gd:givenName"`
	FamilyName string `xml:"gd:familyName"`
}

type organizationMarshal struct {
	Rel            string `xml:"rel,attr"`
	Name           string `xml:"gd:orgName"`
	Title          string `xml:"gd:orgTitle"`
	JobDescription string `xml:"gd:orgJobDescription"`
	Department     string `xml:"gd:orgDepartment"`
}

type whereMarshal struct {
	ValueString string `xml:"valueString,attr"`
}

type notesMarshal struct {
	Type  string `xml:"type,attr"`
	Notes string `xml:",chardata"`
}

// NewGoogleContactsDestination creates a new GoogleContacts instance
func NewGoogleContactsDestination(destinationConfig internal.DestinationConfig) (internal.Destination,
	error,
) {
	if destinationConfig.Type != internal.DestinationTypeGoogleContacts {
		return nil, fmt.Errorf("invalid config type: %s", destinationConfig.Type)
	}

	var googleContacts GoogleContacts
	// Unmarshal ExtraJSON into GoogleConfig struct
	err := json.Unmarshal(destinationConfig.ExtraJSON, &googleContacts.GoogleConfig)
	if err != nil {
		return &GoogleContacts{}, err
	}

	// Defaults
	if googleContacts.BatchSize <= 0 {
		googleContacts.BatchSize = DefaultBatchSize
	}
	if googleContacts.BatchDelaySeconds <= 0 {
		googleContacts.BatchDelaySeconds = DefaultBatchDelaySeconds
	}

	googleContacts.DestinationConfig = destinationConfig

	// Initialize Client object
	err = googleContacts.initGoogleClient()
	if err != nil {
		return &GoogleContacts{}, err
	}

	return &googleContacts, nil
}

// ForSet is not implemented for this destination. Only one sync set may be defined in config.json.
func (g *GoogleContacts) ForSet(syncSetJson json.RawMessage) error {
	// sync sets not implemented for this destination
	return nil
}

// ListUsers returns all users (contacts) in the destination
func (g *GoogleContacts) ListUsers(desiredAttrs []string) ([]internal.Person, error) {
	href := fmt.Sprintf("https://www.google.com/m8/feeds/contacts/%s/full?max-results=%d",
		g.GoogleConfig.Domain, MaxQuerySize)
	body, err := g.httpRequest(http.MethodGet, href, "", map[string]string{})
	if err != nil {
		syncErr := internal.SyncError{
			Message:   fmt.Errorf("failed to retrieve user list: %w", err),
			SendAlert: true,
		}
		if e, ok := err.(*googleapi.Error); ok && e.Code == http.StatusServiceUnavailable {
			syncErr.SendAlert = false
		}
		return []internal.Person{}, syncErr
	}

	var parsed Entries

	if err := xml.Unmarshal([]byte(body), &parsed); err != nil {
		return []internal.Person{}, fmt.Errorf("failed to parse xml for user list: %s", err)
	}
	if parsed.Total >= MaxQuerySize {
		return []internal.Person{}, fmt.Errorf("too many entries in Google Contacts directory")
	}

	return g.extractPersonsFromResponse(parsed.Entries)
}

// ApplyChangeSet executes all of the configured sync tasks (create, update, and/or delete)
func (g *GoogleContacts) ApplyChangeSet(
	changes internal.ChangeSet,
	eventLog chan<- internal.EventLogItem,
) internal.ChangeResults {
	var results internal.ChangeResults
	var wg sync.WaitGroup

	batchTimer := internal.NewBatchTimer(g.BatchSize,
		g.BatchDelaySeconds)

	if g.DestinationConfig.DisableAdd {
		log.Println("Contact creation is disabled.")
	} else {
		for _, toCreate := range changes.Create {
			wg.Add(1)
			go g.addContact(toCreate, &results.Created, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	if g.DestinationConfig.DisableUpdate {
		log.Println("Contact update is disabled.")
	} else {
		for _, toUpdate := range changes.Update {
			wg.Add(1)
			go g.updateContact(toUpdate, &results.Updated, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	if g.DestinationConfig.DisableDelete {
		log.Println("Contact deletion is disabled.")
	} else {
		for _, toUpdate := range changes.Delete {
			wg.Add(1)
			go g.deleteContact(toUpdate, &results.Deleted, &wg, eventLog)
			batchTimer.WaitOnBatch()
		}
	}

	wg.Wait()

	return results
}

func (g *GoogleContacts) httpRequest(verb, url, body string, headers map[string]string) (string,
	error,
) {
	var req *http.Request
	var err error
	if body == "" {
		req, err = http.NewRequest(verb, url, nil)
	} else {
		req, err = http.NewRequest(verb, url, bytes.NewBuffer([]byte(body)))
	}
	if err != nil {
		return "", err
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("GData-Version", "3.0")
	req.Header.Set("User-Agent", "personnel-sync")

	resp, err := g.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read http response body: %s", err)
	}
	bodyString := string(bodyBytes)

	if resp.StatusCode >= 400 {
		return bodyString, errors.New(resp.Status)
	}

	return bodyString, nil
}

func (g *GoogleContacts) extractPersonsFromResponse(contacts []Contact) ([]internal.Person, error) {
	persons := make([]internal.Person, len(contacts))
	for i, entry := range contacts {
		id := findSelfLink(entry)

		attributes := map[string]string{
			contactFieldID:             id,
			contactFieldEmail:          findPrimaryEmail(entry),
			contactFieldFullName:       entry.Title,
			contactFieldGivenName:      entry.Name.GivenName,
			contactFieldFamilyName:     entry.Name.FamilyName,
			contactFieldWhere:          entry.Where.ValueString,
			contactFieldOrganization:   entry.Organization.Name,
			contactFieldTitle:          entry.Organization.Title,
			contactFieldJobDescription: entry.Organization.JobDescription,
			contactFieldDepartment:     entry.Organization.Department,
			contactFieldNotes:          entry.Notes,
		}

		attributes = mergeAttributeMaps(attributes, getPhoneNumbersFromContact(entry))

		persons[i] = internal.Person{
			CompareValue: findPrimaryEmail(entry),
			ID:           id,
			Attributes:   attributes,
		}
	}

	return persons, nil
}

func mergeAttributeMaps(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func findSelfLink(entry Contact) string {
	for _, link := range entry.Links {
		if link.Rel == "self" {
			return link.Href
		}
	}
	return ""
}

func findPrimaryEmail(entry Contact) string {
	for _, email := range entry.Emails {
		if email.Primary {
			return email.Address
		}
	}
	return ""
}

func getPhoneNumbersFromContact(contact Contact) map[string]string {
	y := map[string]string{}

	for _, phone := range contact.PhoneNumbers {
		key := contactFieldPhoneNumber

		// Google supports only `rel` or `label` but not both, so the order here does not matter
		if phone.Rel != "" {
			key += delim + phone.Rel
		}
		if phone.Label != "" {
			key += delim + phone.Label
		}

		y[key] = phone.Value
	}

	y[contactFieldPhoneNumber] = findPrimaryPhoneNumber(contact)

	return y
}

func findPrimaryPhoneNumber(entry Contact) string {
	for _, phone := range entry.PhoneNumbers {
		if phone.Primary {
			return phone.Value
		}
	}
	return ""
}

func (g *GoogleContacts) addContact(
	person internal.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- internal.EventLogItem,
) {
	defer wg.Done()

	href := "https://www.google.com/m8/feeds/contacts/" + g.GoogleConfig.Domain + "/full"
	body, err := g.createBody(person)
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level: syslog.LOG_ERR,
			Message: fmt.Sprintf("error creating addContact request for '%s' in Google contacts: %s",
				person.CompareValue, err),
		}
		return
	}

	headers := map[string]string{"Content-Type": "application/atom+xml"}
	if _, err := g.httpRequest(http.MethodPost, href, body, headers); err != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("unable to insert %s in Google contacts: %s", person.CompareValue, err),
		}
		return
	}

	eventLog <- internal.EventLogItem{
		Level:   syslog.LOG_INFO,
		Message: "AddContact " + person.CompareValue,
	}

	atomic.AddUint64(counter, 1)
}

// initGoogleClient creates an http Client and adds a JWT config that has the required OAuth 2.0 scopes
//
// Authentication requires an email address that matches an actual GMail user (e.g. a machine account)
// that has appropriate access privileges
func (g *GoogleContacts) initGoogleClient() error {
	googleAuthJson, err := json.Marshal(g.GoogleConfig.GoogleAuth)
	if err != nil {
		return fmt.Errorf("unable to marshal google auth data into json, error: %s", err)
	}

	config, err := google.JWTConfigFromJSON(googleAuthJson, "https://www.google.com/m8/feeds/contacts/")
	if err != nil {
		return fmt.Errorf("unable to parse client secret file to config: %s", err)
	}

	config.Subject = g.GoogleConfig.DelegatedAdminEmail
	g.Client = *config.Client(context.Background())

	return nil
}

// createBody inserts attributes into an XML request body.
//
// WARNING: This updates all fields, even if omitted in the field mapping. A safer implementation would be to
// merge the data retrieved from Google with the data coming from the source.
func (g *GoogleContacts) createBody(person internal.Person) (string, error) {
	contact := contactMarshal{
		XmlNSAtom: "http://www.w3.org/2005/Atom",
		XmlNSGd:   "http://schemas.google.com/g/2005",
		AtomCategory: atomCategory{
			Scheme: "http://schemas.google.com/g/2005#kind",
			Term:   "http://schemas.google.com/contact/2008#contact",
		},
		Name: nameMarshal{
			FullName:   person.Attributes[contactFieldFullName],
			GivenName:  person.Attributes[contactFieldGivenName],
			FamilyName: person.Attributes[contactFieldFamilyName],
		},
		Emails: []emailMarshal{
			{
				Rel:     relPhoneWork,
				Primary: true,
				Address: person.Attributes[contactFieldEmail],
			},
		},
		Organization: organizationMarshal{
			Rel:            relPhoneWork,
			Name:           person.Attributes[contactFieldOrganization],
			Title:          person.Attributes[contactFieldTitle],
			JobDescription: person.Attributes[contactFieldJobDescription],
			Department:     person.Attributes[contactFieldDepartment],
		},
		Where: whereMarshal{
			ValueString: person.Attributes[contactFieldWhere],
		},
		Notes: notesMarshal{
			Type:  "text",
			Notes: person.Attributes[contactFieldNotes],
		},
	}

	contact.PhoneNumbers = getPhonesFromAttributes(person.Attributes)

	output, err := xml.Marshal(&contact)

	// For debug, this can be used to improve XML readability
	// output, err := xml.MarshalIndent(&contact, "", "  ")

	return string(output), err
}

func getPhonesFromAttributes(attributes map[string]string) []phoneNumberMarshal {
	var phones []phoneNumberMarshal
	for key, val := range attributes {
		if key == contactFieldPhoneNumber {
			phones = append(phones, phoneNumberMarshal{
				Rel:     relPhoneWork,
				Primary: true,
				Value:   val,
			})
			continue
		}
		if !strings.HasPrefix(key, contactFieldPhoneNumber) {
			continue
		}
		split := strings.Split(key, delim)
		if len(split) < 2 {
			continue
		}
		label := ""
		rel := ""
		if strings.HasPrefix(split[1], "http") {
			rel = split[1]
		} else {
			label = split[1]
		}
		phones = append(phones, phoneNumberMarshal{
			Rel:     rel,
			Primary: false,
			Label:   label,
			Value:   val,
		})
	}
	return phones
}

func (g *GoogleContacts) updateContact(
	person internal.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- internal.EventLogItem,
) {
	defer wg.Done()

	url := person.ID

	contact, err := g.getContact(url)
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("failed retrieving contact %s: %s", person.CompareValue, err),
		}
		return
	}

	body, err := g.createBody(person)
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level: syslog.LOG_ERR,
			Message: fmt.Sprintf("error creating updateContact request for '%s' in Google contacts: %s",
				person.CompareValue, err),
		}
		return
	}

	_, err = g.httpRequest(http.MethodPut, url, body, map[string]string{
		"If-Match":     contact.Etag,
		"Content-Type": "application/atom+xml",
	})
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("updateContact failed updating user %s: %s", person.CompareValue, err),
		}
		return
	}

	atomic.AddUint64(counter, 1)
}

func (g *GoogleContacts) getContact(url string) (Contact, error) {
	existingContact, err := g.httpRequest(http.MethodGet, url, "", map[string]string{})
	if err != nil {
		return Contact{}, fmt.Errorf("GET failed: %s", err)
	}

	var c Contact
	err = xml.Unmarshal([]byte(existingContact), &c)
	if err != nil {
		return Contact{}, fmt.Errorf("failed to parse xml: %s", err)
	}

	return c, nil
}

func (g *GoogleContacts) deleteContact(
	person internal.Person,
	counter *uint64,
	wg *sync.WaitGroup,
	eventLog chan<- internal.EventLogItem,
) {
	defer wg.Done()

	url := person.ID

	contact, err := g.getContact(url)
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("failed retrieving contact %s: %s", person.CompareValue, err),
		}
		return
	}

	_, err = g.httpRequest(http.MethodDelete, url, "", map[string]string{
		"If-Match": contact.Etag,
	})
	if err != nil {
		eventLog <- internal.EventLogItem{
			Level:   syslog.LOG_ERR,
			Message: fmt.Sprintf("deleteContact failed deleting user %s: %s", person.CompareValue, err),
		}
		return
	}

	atomic.AddUint64(counter, 1)
}
