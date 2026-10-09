package restapi

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strconv"
)

type fakeEndpoint struct {
	path             string
	method           string
	status           int
	responseBody     string
	authType         string
	username         string
	password         string
	compareAttr      string
	idAttr           string
	resultsContainer string
}

const (
	EndpointListWorkday    = "list workday"
	EndpointListOther      = "list other"
	EndpointListSalesforce = "list salesforce"
	EndpointCreateOther    = "create other"
)

const extraJSONtemplate = `{
  "ListMethod": "%s",
  "BaseURL": "%s",
  "ResultsJSONContainer": "%s",
  "AuthType": "%s",
  "Username": "%s",
  "Password": "%s",
  "CompareAttribute": "%s",
  "IDAttribute": "%s"
}`

const workdayUsersJSON = `{
"root": {  
  "Report_Entry": [
    {
      "Employee_Number": "10013",
      "First_Name": "Mickey",
      "Last_Name": "Mouse",
      "Display_Name": "Mickey Mouse",
      "Username": "MICKEY_MOUSE",
      "Email": "mickey_mouse@acme.com",
      "Personal_Email": "mickey_mouse@mousemail.com",
      "Account_Locked__Disabled_or_Expired": "0",
      "requireMfa": "0",
      "Company": "Disney"
    },
	{
      "Employee_Number": "10011",
      "First_Name": "Donald",
      "Last_Name": "Duck",
      "Display_Name": "Donald Duck",
      "Username": "DONALD_DUCK",
      "Email": "donald_duck@acme.com",
      "Personal_Email": "donald_duck@duckmail.com",
      "Account_Locked__Disabled_or_Expired": "0",
      "requireMfa": "0",
      "Company": "Disney"
    }
  ]
}}`

const otherUsersJSON = `[
    {
      "employeeID": 10000013,
      "first": "Mickey",
      "last": "Mouse",
      "display": "Mickey Mouse",
      "username": "MICKEY_MOUSE",
      "email": "mickey_mouse@acme.com"
    },
	{
      "employeeID": 10000011,
      "first": "Donald",
      "last": "Duck",
      "display": "Donald Duck",
      "username": "DONALD_DUCK",
      "email": "donald_duck@acme.com"
    }
]`

const salesforceUsersJSON = `{
  "totalSize": 2,
  "done": true,
  "records": [
    {
      "attributes": {
        "type": "fHCM2__Team_Member__c",
        "url": "/services/data/v20.0/sobjects/fHCM2__Team_Member__c/a1H1U737901ULOwUAO"
      },
      "Name": "Mickey Mouse",
      "fHCM2__User__r": {
        "attributes": {
          "type": "User",
          "url": "/services/data/v20.0/sobjects/User/0051U579303drCrQAI"
        },
        "Email": "mickey_mouse@acme.com"
      }
    },
    {
      "attributes": {
        "type": "fHCM2__Team_Member__c",
        "url": "/services/data/v20.0/sobjects/fHCM2__Team_Member__c/a1H1U50361ULZbUAO"
      },
      "Name": "Donald Duck",
      "fHCM2__User__r": {
        "attributes": {
          "type": "User",
          "url": "/services/data/v20.0/sobjects/User/0051U773763dqt3QAA"
        },
        "Email": "donald_duck@acme.com"
      }
    }
  ]
}`

func getFakeEndpoints() map[string]fakeEndpoint {
	return map[string]fakeEndpoint{
		EndpointListWorkday: {
			path:             "/workday",
			method:           http.MethodGet,
			status:           http.StatusOK,
			responseBody:     workdayUsersJSON,
			authType:         AuthTypeBasic,
			username:         "workday_username",
			password:         "workday_password",
			compareAttr:      "Email",
			resultsContainer: "root.Report_Entry",
		},
		EndpointListOther: {
			path:             "/other/list",
			method:           http.MethodGet,
			status:           http.StatusOK,
			responseBody:     otherUsersJSON,
			authType:         AuthTypeBearer,
			password:         "bearer_token",
			compareAttr:      "email",
			idAttr:           "employeeID",
			resultsContainer: "",
		},
		EndpointListSalesforce: {
			path:             "/sfdc",
			method:           http.MethodGet,
			status:           http.StatusOK,
			responseBody:     salesforceUsersJSON,
			authType:         AuthTypeSalesforceOauth,
			username:         "sf_username",
			password:         "sf_token",
			compareAttr:      "fHCM2__User__r.Email",
			resultsContainer: "records",
		},
		EndpointCreateOther: {
			path:             "/other/create",
			method:           http.MethodPost,
			status:           http.StatusOK,
			authType:         AuthTypeBearer,
			password:         "bearer_token",
			compareAttr:      "",
			resultsContainer: "",
		},
	}
}

func getTestServer() *httptest.Server {
	mux := http.NewServeMux()
	endpoints := getFakeEndpoints()
	for name := range endpoints {
		e := endpoints[name]
		status := e.status
		responseBody := e.responseBody
		mux.HandleFunc(e.path, func(w http.ResponseWriter, req *http.Request) {
			switch e.authType {
			case AuthTypeBasic:
				user, pass, ok := req.BasicAuth()
				if !ok || e.username != user || e.password != pass {
					status = http.StatusUnauthorized
					responseBody = `{"error": "Not Authorized"}`
				}
			case AuthTypeBearer, AuthTypeSalesforceOauth:
				token := req.Header.Get("Authorization")
				if "Bearer "+e.password != token {
					status = http.StatusUnauthorized
					responseBody = `{"error": "Not Authorized"}`
				}
			}

			// basic check to see if a POST has a request body
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				status = http.StatusBadRequest
			}
			bodyString := string(bodyBytes)
			if req.Method == http.MethodPost && bodyString == "" {
				status = http.StatusBadRequest
				responseBody = `{"error":"empty request body"}`
			}

			pageParam := req.URL.Query()["page"]
			if len(pageParam) > 0 && pageParam[0] != "1" {
				responseBody = "{}"
			}

			w.WriteHeader(status)
			w.Header().Set("content-type", "application/json")
			_, _ = io.WriteString(w, responseBody)
		})
	}
	return httptest.NewServer(mux)
}

// Used for mock User results
type MockUser struct {
	ID   int
	Name string
}

type PaginationConfig struct {
	numberKey string // the qs param key for the next set to get
	sizeKey   string // the qs param key for the size of each set
	usersJSON string // json encoded MockUser objects
	users     []MockUser
}

const (
	itemsQSParam = "itemsRequest" // use this to signify item based pagination requests
	pagesQSParam = "pagesRequest" // use this to signify page based pagination requests
)

func getTestPaginationServer(path string, pConfig PaginationConfig) *httptest.Server {
	mux := http.NewServeMux()

	responseBody := "[]"

	mux.HandleFunc(path, func(w http.ResponseWriter, req *http.Request) {
		qsParams := req.URL.Query()
		startIndexStr, ok := qsParams[pConfig.numberKey]
		startIndex := 0
		var err error

		if ok {
			startIndex, err = strconv.Atoi(startIndexStr[0])
			if err != nil {
				log.Panicf("incorrect start index query string param in test: %v", startIndexStr)
			}
		}

		// convert the size string to an integer
		sizeStr, ok := qsParams[pConfig.sizeKey]
		count := 999
		if ok {
			count, err = strconv.Atoi(sizeStr[0])
			if err != nil {
				log.Panicf("incorrect size query string param in test: %v", sizeStr)
			}
		}

		if _, ok := qsParams[itemsQSParam]; ok {
			responseBody = getItemBasedUsers(startIndex, count, pConfig)
		} else {
			if _, ok := qsParams[pagesQSParam]; ok {
				responseBody = getPageBasedUsers(startIndex, count, pConfig)
			} else { // no pagination requested
				responseBody = pConfig.usersJSON
			}
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w, responseBody)
	})
	return httptest.NewServer(mux)
}

func getItemBasedUsers(startIndex, count int, pConfig PaginationConfig) string {
	userCount := len(pConfig.users)

	responseBody := `[]`
	if startIndex >= userCount {
		return responseBody
	}

	endIndex := min(startIndex+count, userCount)

	reqUsers := pConfig.users[startIndex:endIndex]

	usersJSON, err := json.Marshal(reqUsers)
	if err != nil {
		log.Panicf("error marshalling users output: %s", err)
	}
	responseBody = string(usersJSON)
	return responseBody
}

func getPageBasedUsers(startPage, count int, pConfig PaginationConfig) string {
	userCount := len(pConfig.users)

	responseBody := `[]`
	startIndex := max((startPage-1)*count, 0)

	if startIndex >= userCount {
		return responseBody
	}

	endIndex := min(startIndex+count, userCount)

	reqUsers := pConfig.users[startIndex:endIndex]

	usersJSON, err := json.Marshal(reqUsers)
	if err != nil {
		log.Panicf("error marshalling users output: %s", err)
	}
	responseBody = string(usersJSON)
	return responseBody
}
